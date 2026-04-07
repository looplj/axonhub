#!/usr/bin/env node

const https = require('https');
const http = require('http');
const fs = require('fs');
const path = require('path');
const { URL } = require('url');

const SOURCE_URL = 'https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/refs/heads/dev/dist/all.json';
const CONSTANTS_PATH = path.join(__dirname, '../../frontend/src/features/models/data/constants.ts');
const OUTPUT_PATH = path.join(__dirname, '../../frontend/src/features/models/data/providers.json');
const MODELS_JSON_PATH = path.join(__dirname, './models.json');

function fetchJSON(url) {
  return new Promise((resolve, reject) => {
    const proxyUrl = process.env.HTTPS_PROXY || process.env.https_proxy ||
                     process.env.HTTP_PROXY || process.env.http_proxy;

    if (!proxyUrl) {
      // 无代理,直接请求
      https.get(url, (res) => {
        let data = '';
        res.on('data', (chunk) => { data += chunk; });
        res.on('end', () => {
          try {
            resolve(JSON.parse(data));
          } catch (e) {
            reject(new Error(`Failed to parse JSON: ${e.message}`));
          }
        });
      }).on('error', reject);
      return;
    }

    // 使用代理
    console.log('Using proxy:', proxyUrl);
    const targetUrl = new URL(url);
    const proxy = new URL(proxyUrl);

    const connectOptions = {
      method: 'CONNECT',
      host: proxy.hostname,
      port: proxy.port || 80,
      path: `${targetUrl.hostname}:443`,
      headers: { Host: targetUrl.hostname }
    };

    // 添加代理认证(如果有)
    if (proxy.username || proxy.password) {
      const auth = Buffer.from(`${proxy.username}:${proxy.password}`).toString('base64');
      connectOptions.headers['Proxy-Authorization'] = `Basic ${auth}`;
    }

    const connectReq = http.request(connectOptions);

    connectReq.on('connect', (res, socket) => {
      if (res.statusCode !== 200) {
        return reject(new Error(`Proxy CONNECT failed: ${res.statusCode}`));
      }

      const tlsOptions = {
        socket: socket,
        hostname: targetUrl.hostname,
        path: targetUrl.pathname + targetUrl.search,
        method: 'GET'
      };

      const tlsReq = https.request(tlsOptions, (tlsRes) => {
        let data = '';
        tlsRes.on('data', (chunk) => { data += chunk; });
        tlsRes.on('end', () => {
          try {
            resolve(JSON.parse(data));
          } catch (e) {
            reject(new Error(`Failed to parse JSON: ${e.message}`));
          }
        });
      });

      tlsReq.on('error', reject);
      tlsReq.end();
    });

    connectReq.on('error',reject);
    connectReq.end();
  });
}

function extractDeveloperIds(constantsPath) {
  const content = fs.readFileSync(constantsPath, 'utf8');
  const match = content.match(/export const DEVELOPER_IDS = \[([\s\S]*?)\]/);

  if (!match) {
    throw new Error('Could not find DEVELOPER_IDS in constants.ts');
  }

  const idsString = match[1];
  const ids = idsString
    .split(',')
    .map(line => line.trim())
    .filter(line => line.startsWith("'") || line.startsWith('"'))
    .map(line => line.replace(/^['"]|['"]$/g, ''));

  return ids;
}

function filterProviders(data, allowedIds) {
  if (!data.providers) {
    throw new Error('Invalid data structure: missing providers field');
  }

  const filtered = {};

  for (const [key, value] of Object.entries(data.providers)) {
    if (allowedIds.includes(value.id)) {
      filtered[key] = value;
    }
  }

  // Map doubao channel's doubao models to bytedance developer
  if (allowedIds.includes('bytedance') && data.providers['doubao']) {
    const doubaoProvider = data.providers['doubao'];
    const doubaoModels = (doubaoProvider.models || []).filter(m =>
      m.id && m.id.toLowerCase().startsWith('doubao')
    );
    if (doubaoModels.length > 0) {
      filtered['bytedance'] = {
        ...doubaoProvider,
        id: 'bytedance',
        name: 'ByteDance',
        display_name: 'ByteDance',
        models: doubaoModels,
      };
      console.log(`Mapped ${doubaoModels.length} doubao models to bytedance developer`);
    }
  }

  // Filter NVIDIA models to only include NVIDIA-created models (nvidia/* prefix)
  if (allowedIds.includes('nvidia') && filtered['nvidia']) {
    const nvidiaProvider = filtered['nvidia'];
    const nvidiaModels = (nvidiaProvider.models || []).filter(m =>
      m.id && m.id.toLowerCase().startsWith('nvidia/')
    );
    if (nvidiaModels.length > 0) {
      filtered['nvidia'] = {
        ...nvidiaProvider,
        models: nvidiaModels,
      };
      console.log(`Filtered ${nvidiaModels.length} NVIDIA-created models from ${nvidiaProvider.models.length} total`);
    }
  }

  return { providers: filtered };
}

function sortModelsByDate(data) {
  for (const provider of Object.values(data.providers)) {
    if (provider.models && Array.isArray(provider.models)) {
      provider.models.sort((a, b) => {
        const dateA = a.release_date ? new Date(a.release_date) : new Date(0);
        const dateB = b.release_date ? new Date(b.release_date) : new Date(0);
        return dateB - dateA;
      });
    }
  }
  return data;
}

function mergeWithModelsJson(data, modelsJsonPath) {
  if (!fs.existsSync(modelsJsonPath)) {
    console.log('models.json does not exist, skipping merge');
    return data;
  }

  console.log('Merging with models.json...');
  const modelsJson = JSON.parse(fs.readFileSync(modelsJsonPath, 'utf8'));

  for (const [providerKey, models] of Object.entries(modelsJson)) {
    if (data.providers[providerKey]) {
      const existingProvider = data.providers[providerKey];
      if (!existingProvider.models) {
        existingProvider.models = [];
      }

      const existingIds = new Set(existingProvider.models.map(m => m.id));

      for (const model of models) {
        if (!existingIds.has(model.id)) {
          existingProvider.models.push(model);
          existingIds.add(model.id);
        }
      }
    } else {
      data.providers[providerKey] = {
        id: providerKey,
        models: models
      };
    }
  }

  return data;
}

async function main() {
  try {
    console.log('Fetching model developers data from:', SOURCE_URL);
    const data = await fetchJSON(SOURCE_URL);

    console.log('Extracting allowed developer IDs from:', CONSTANTS_PATH);
    const allowedIds = extractDeveloperIds(CONSTANTS_PATH);
    console.log('Allowed developer IDs:', allowedIds);

    console.log('Filtering providers...');
    const filtered = filterProviders(data, allowedIds);

    const providerCount = Object.keys(filtered.providers).length;
    console.log(`Filtered to ${providerCount} providers`);

    console.log('Merging with models.json...');
    mergeWithModelsJson(filtered, MODELS_JSON_PATH);

    console.log('Sorting models by release date...');
    sortModelsByDate(filtered);

    console.log('Writing to:', OUTPUT_PATH);
    fs.writeFileSync(OUTPUT_PATH, JSON.stringify(filtered, null, 2) + '\n');

    console.log('Sync completed successfully!');
  } catch (error) {
    console.error('Error during sync:', error.message);
    process.exit(1);
  }
}

main();
