import { createServer, type Server } from 'node:http'
import { expect, type APIRequestContext, type Page } from '@playwright/test'
import { gotoAndEnsureAuth, signInAsAdmin } from './auth.utils'

// Type declaration for process
declare const process: {
  env: Record<string, string | undefined>
}

export const suffix = Date.now().toString().slice(-6)
// Synthetic markers that must never reach the browser: the channel credential and the upstream error body.
const CHANNEL_KEY_CANARY = `codex-key-canary-${suffix}`
const UPSTREAM_BODY_CANARY = `codex-upstream-body-canary-${suffix}`
const OWNER = {
  email: process.env.AXONHUB_ADMIN_EMAIL || 'my@example.com',
  password: process.env.AXONHUB_ADMIN_PASSWORD || 'pwd123456',
}

type ChannelStatus = 'enabled' | 'disabled' | 'archived'

interface Fixture {
  server: Server
  upstreamURL: string
  ownerToken: string
  channelGuids: string[]
}

let fixture: Fixture | undefined

function current(): Fixture {
  if (!fixture) throw new Error('startFixture() has not run')
  return fixture
}

async function listenFixtureUpstream(): Promise<{ server: Server; upstreamURL: string }> {
  const server = createServer((req, res) => {
    const path = new URL(req.url ?? '/', 'http://fixture').pathname
    const send = (status: number, body: unknown, delayMs = 0) =>
      setTimeout(() => {
        res.writeHead(status, { 'content-type': 'application/json' })
        res.end(JSON.stringify(body))
      }, delayMs)
    if (path === '/ok/models') return send(200, { models: [{ slug: 'pw-model-1' }, { slug: 'pw-model-2' }, { slug: 'pw-model-3' }] })
    if (path === '/slow/models') return send(200, { models: [{ slug: 'pw-slow-1' }] }, 2500)
    if (path === '/missing/models') return send(404, { error: UPSTREAM_BODY_CANARY })
    return send(404, { error: 'unknown fixture path' })
  })
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  const address = server.address()
  if (address === null || typeof address === 'string') throw new Error('fixture upstream has no TCP address')
  return { server, upstreamURL: `http://127.0.0.1:${address.port}` }
}

export async function graphql<T>(request: APIRequestContext, query: string, variables: Record<string, unknown> = {}): Promise<T> {
  const response = await request.post('/admin/graphql', {
    headers: { Authorization: `Bearer ${current().ownerToken}` },
    data: { query, variables },
  })
  const body = await response.json()
  if (body.errors) throw new Error(JSON.stringify(body.errors))
  return body.data as T
}

export async function saveCodexSettings(request: APIRequestContext, input: { enabled: boolean; channelID: number | null }): Promise<void> {
  await graphql(request, `mutation ($input: UpdateCodexCompatibilitySettingsInput!) { updateCodexCompatibilitySettings(input: $input) }`, { input })
}

// Starts the synthetic upstream (ok, slow, and 404 catalogs) and signs in as the owner for GraphQL seeding.
export async function startFixture(request: APIRequestContext): Promise<void> {
  const { server, upstreamURL } = await listenFixtureUpstream()
  const signin = await request.post('/admin/auth/signin', { data: OWNER })
  expect(signin.ok()).toBeTruthy()
  fixture = { server, upstreamURL, ownerToken: (await signin.json()).token as string, channelGuids: [] }
  await saveCodexSettings(request, { enabled: false, channelID: null })
}

export async function stopFixture(request: APIRequestContext): Promise<void> {
  const { server, channelGuids } = current()
  await saveCodexSettings(request, { enabled: false, channelID: null })
  await graphql(request, `mutation ($ids: [ID!]!) { bulkArchiveChannels(ids: $ids) }`, { ids: channelGuids })
  server.closeAllConnections()
  server.close()
}

interface ChannelSpec {
  name: string
  path: string
  status: ChannelStatus
  type?: 'codex' | 'openai'
}

export async function createChannel(request: APIRequestContext, spec: ChannelSpec): Promise<{ guid: string; id: number }> {
  const { upstreamURL, channelGuids } = current()
  const created = await graphql<{ createChannel: { id: string } }>(request, `mutation ($input: CreateChannelInput!) { createChannel(input: $input) { id } }`, {
    input: {
      type: spec.type ?? 'codex',
      name: spec.name,
      baseURL: `${upstreamURL}${spec.path}`,
      credentials: { apiKey: CHANNEL_KEY_CANARY },
      supportedModels: ['pw-model-1'],
      defaultTestModel: 'pw-model-1',
    },
  })
  const guid = created.createChannel.id
  if (spec.status !== 'disabled') {
    await graphql(request, `mutation ($id: ID!, $status: ChannelStatus!) { updateChannelStatus(id: $id, status: $status) { id } }`, { id: guid, status: spec.status })
  }
  channelGuids.push(guid)
  return { guid, id: Number(guid.split('/').pop()) }
}

export async function createUser(request: APIRequestContext, email: string, scopes: string[]): Promise<void> {
  await graphql(request, `mutation ($input: CreateUserInput!) { createUser(input: $input) { id } }`, {
    input: { email, password: 'pwd123456', firstName: 'Codex', lastName: 'Scopes', scopes },
  })
}

// Records every GraphQL exchange and browser error so each scenario can prove no secret or error leaked.
export function watch(page: Page) {
  const exchanges: { operation: string; variables: unknown; response: string }[] = []
  const errors: string[] = []
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(message.text())
  })
  page.on('pageerror', (error) => errors.push(error.message))
  page.on('response', async (response) => {
    if (!response.url().includes('/admin/graphql')) return
    const request = response.request().postDataJSON()
    exchanges.push({ operation: request?.operationName ?? '', variables: request?.variables, response: await response.text().catch(() => '') })
  })
  return { exchanges, errors }
}

export function operationResponse(page: Page, operation: string) {
  return page.waitForResponse((response) => response.url().includes('/admin/graphql') && response.request().postDataJSON()?.operationName === operation)
}

export async function expectNoLeak(page: Page, watched: ReturnType<typeof watch>) {
  const dom = await page.locator('body').innerText()
  for (const canary of [CHANNEL_KEY_CANARY, UPSTREAM_BODY_CANARY]) {
    expect(dom).not.toContain(canary)
    for (const exchange of watched.exchanges) expect(exchange.response).not.toContain(canary)
  }
  expect(watched.errors).toEqual([])
}

// Signs in first and attaches the watchers afterwards so sign-in noise is not recorded.
export async function openCodexTab(page: Page) {
  await gotoAndEnsureAuth(page, '/system?tab=codex')
  const watched = watch(page)
  await page.reload()
  await expect(page.getByTestId('codex-compat-card')).toBeVisible()
  return watched
}

export async function openCodexTabAs(page: Page, email: string) {
  await page.goto('/sign-in')
  await signInAsAdmin(page, { email, password: 'pwd123456' })
  const watched = watch(page)
  await page.goto('/system?tab=codex')
  await expect(page.getByTestId('codex-compat-card')).toBeVisible()
  return watched
}

export async function saveSettings(page: Page) {
  const saved = operationResponse(page, 'UpdateCodexCompatibilitySettings')
  await page.getByTestId('codex-compat-save-button').click()
  expect((await (await saved).json()).data.updateCodexCompatibilitySettings).toBe(true)
}

export async function chooseChannel(page: Page, name: string) {
  await page.getByTestId('codex-compat-channel-select').click()
  await page.getByRole('option', { name: new RegExp(name) }).click()
}
