export interface CodexCompatibilityExampleBlock {
  key: 'codexCompatibilityConfig' | 'codexCompatibilityAuth' | 'codexCompatibilityEnv';
  language: 'toml' | 'json' | 'bash';
  display: string;
  real: string;
}

export interface CodexCompatibilityExamples {
  baseUrl: string;
  configToml: CodexCompatibilityExampleBlock;
  authJson: CodexCompatibilityExampleBlock;
  shellEnv: CodexCompatibilityExampleBlock;
}

export function buildCodexCompatibilityExamples({
  serverURL,
  apiKey,
  maskedApiKey,
}: {
  serverURL: string;
  apiKey: string;
  maskedApiKey: string;
}): CodexCompatibilityExamples {
  const baseUrl = `${serverURL}/codex`;
  const configTomlText = `model = "gpt-6.1-sol"
model_provider = "openai"
openai_base_url = ${JSON.stringify(baseUrl)}
chatgpt_base_url = ${JSON.stringify(baseUrl)}
web_search = "live"

[features]
fast_mode = true
context_management = true`;

  const authDisplay = JSON.stringify(
    {
      personal_access_token: maskedApiKey,
      OPENAI_API_KEY: maskedApiKey,
    },
    null,
    2
  );
  const authReal = JSON.stringify(
    {
      personal_access_token: apiKey,
      OPENAI_API_KEY: apiKey,
    },
    null,
    2
  );

  const shellEnvText = `# In the same shell that starts Codex (macOS/Linux)
export CODEX_AUTHAPI_BASE_URL=${JSON.stringify(baseUrl)}
# Windows: add CODEX_AUTHAPI_BASE_URL=${baseUrl} in System Environment Variables`;

  return {
    baseUrl,
    configToml: {
      key: 'codexCompatibilityConfig',
      language: 'toml',
      display: configTomlText,
      real: configTomlText,
    },
    authJson: {
      key: 'codexCompatibilityAuth',
      language: 'json',
      display: authDisplay,
      real: authReal,
    },
    shellEnv: {
      key: 'codexCompatibilityEnv',
      language: 'bash',
      display: shellEnvText,
      real: shellEnvText,
    },
  };
}
