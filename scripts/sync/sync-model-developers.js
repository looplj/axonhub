#!/usr/bin/env node

const https = require('https');
const fs = require('fs');
const path = require('path');

const SOURCE_URL = 'https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/refs/heads/dev/dist/all.json';
const CONSTANTS_PATH = path.join(__dirname, '../../frontend/src/features/models/data/constants.ts');
const OUTPUT_PATH = path.join(__dirname, '../../frontend/src/features/models/data/providers.json');
const MODELS_JSON_PATH = path.join(__dirname, './models.json');

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
