#!/usr/bin/env node

const https = require('https');
const fs = require('fs');
const path = require('path');

const SOURCE_URL = 'https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/refs/heads/dev/dist/aihubmix.json';
const CONSTANTS_PATH = path.join(__dirname, '../../frontend/src/features/models/data/constants.ts');
const MODELS_JSON_PATH = path.join(__dirname, './models.json');

// Model id prefix to developer mapping (order matters, first match wins)
const DEVELOPER_PREFIXES = [
  { prefixes: ['gemini', 'imagen'], developer: 'google' },
  { prefixes: ['gpt-image', 'gpt-4o-image', 'dall-e'], developer: 'openai' },
  { prefixes: ['qwen-image'], developer: 'alibaba' },
  { prefixes: ['doubao'], developer: 'bytedance' },
  { prefixes: ['flux', 'FLUX'], developer: 'black-forest-labs' },
  { prefixes: ['V_', 'V3', 'DESCRIBE', 'UPSCALE'], developer: 'ideogram' },
  { prefixes: ['ernie-irag', 'irag'], developer: 'baidu' },
  { prefixes: ['Stable-Diffusion', 'stable-diffusion', 'sdxl', 'sd3'], developer: 'stability' },
];

// Skip models with vip/web variant prefixes
const SKIP_PATTERNS = [/^web-/, /-vip$/];

function fetchJSON(url) {
  return new Promise((resolve, reject) => {
    https.get(url, (res) => {
      let data = '';

      res.on('data', (chunk) => {
        data += chunk;
      });

      res.on('end', () => {
        try {
          resolve(JSON.parse(data));
        } catch (e) {
          reject(new Error(`Failed to parse JSON: ${e.message}`));
        }
      });
    }).on('error', (err) => {
      reject(err);
    });
  });
}

function extractDeveloperIds(constantsPath) {
  const content = fs.readFileSync(constantsPath, 'utf8');
  const match = content.match(/export const DEVELOPER_IDS = \[([\s\S]*?)\]/);

  if (!match) {
    throw new Error('Could not find DEVELOPER_IDS in constants.ts');
  }

  return match[1]
    .split(',')
    .map(line => line.trim())
    .filter(line => line.startsWith("'") || line.startsWith('"'))
    .map(line => line.replace(/^['"]|['"]$/g, ''));
}

function shouldSkip(modelId) {
  return SKIP_PATTERNS.some(pattern => pattern.test(modelId));
}

function matchDeveloper(modelId) {
  for (const { prefixes, developer } of DEVELOPER_PREFIXES) {
    for (const prefix of prefixes) {
      if (modelId.startsWith(prefix)) {
        return developer;
      }
    }
  }
  return null;
}

async function main() {
  try {
    console.log('Fetching image models from:', SOURCE_URL);
    const data = await fetchJSON(SOURCE_URL);

    console.log('Extracting allowed developer IDs from:', CONSTANTS_PATH);
    const allowedIds = extractDeveloperIds(CONSTANTS_PATH);
    console.log('Allowed developer IDs:', allowedIds);

    const allModels = data.models || [];
    const imageModels = allModels.filter(m => m.type === 'image-generation');
    console.log(`Found ${imageModels.length} image-generation models`);

    // Group by developer
    const grouped = {};
    const skipped = [];
    const unmapped = [];

    for (const model of imageModels) {
      if (shouldSkip(model.id)) {
        skipped.push(model.id);
        continue;
      }

      const developer = matchDeveloper(model.id);
      if (!developer) {
        unmapped.push(model.id);
        continue;
      }

      if (!allowedIds.includes(developer)) {
        skipped.push(`${model.id} (developer: ${developer})`);
        continue;
      }

      if (!grouped[developer]) {
        grouped[developer] = [];
      }
      grouped[developer].push(model);
    }

    if (skipped.length > 0) {
      console.log('Skipped models:', skipped);
    }

    if (unmapped.length > 0) {
      console.log('Unmapped models (skipped):', unmapped);
    }

    for (const [dev, models] of Object.entries(grouped)) {
      console.log(`  ${dev}: ${models.length} models`);
    }

    // Read existing models.json
    let existing = {};
    if (fs.existsSync(MODELS_JSON_PATH)) {
      existing = JSON.parse(fs.readFileSync(MODELS_JSON_PATH, 'utf8'));
      console.log('Loaded existing models.json');
    }

    // Merge: new models are added, existing models only get missing fields filled
    let addedCount = 0;
    let updatedCount = 0;
    for (const [developer, models] of Object.entries(grouped)) {
      if (!existing[developer]) {
        existing[developer] = [];
      }

      const existingMap = new Map(existing[developer].map(m => [m.id, m]));

      for (const model of models) {
        const existingModel = existingMap.get(model.id);
        if (!existingModel) {
          existing[developer].push(model);
          existingMap.set(model.id, model);
          addedCount++;
        } else {
          // Fill missing fields only
          let changed = false;
          for (const [key, value] of Object.entries(model)) {
            if (existingModel[key] === undefined || existingModel[key] === null) {
              existingModel[key] = value;
              changed = true;
            }
          }
          if (changed) updatedCount++;
        }
      }
    }

    console.log(`Added ${addedCount} new models, updated ${updatedCount} existing models`);

    fs.writeFileSync(MODELS_JSON_PATH, JSON.stringify(existing, null, 2) + '\n');
    console.log('Written to:', MODELS_JSON_PATH);
    console.log('Sync completed successfully!');
  } catch (error) {
    console.error('Error during sync:', error.message);
    process.exit(1);
  }
}

main();
