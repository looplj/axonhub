const fs = require('fs');
const path = require('path');

const localesDir = path.join(__dirname, '../frontend/src/locales');

function flattenObject(obj, prefix = '') {
  return Object.keys(obj).reduce((acc, key) => {
    const pre = prefix.length ? prefix + '.' : '';
    if (typeof obj[key] === 'object' && obj[key] !== null && !Array.isArray(obj[key])) {
      Object.assign(acc, flattenObject(obj[key], pre + key));
    } else {
      acc[pre + key] = obj[key];
    }
    return acc;
  }, {});
}

function processDirectory(dir) {
  const files = fs.readdirSync(dir);

  for (const file of files) {
    const fullPath = path.join(dir, file);
    const stat = fs.statSync(fullPath);

    if (stat.isDirectory()) {
      processDirectory(fullPath);
    } else if (file.endsWith('.json')) {
      console.log(`Processing ${fullPath}...`);
      try {
        const content = JSON.parse(fs.readFileSync(fullPath, 'utf8'));
        const flattened = flattenObject(content);
        fs.writeFileSync(fullPath, JSON.stringify(flattened, null, 2) + '\n');
        console.log(`Successfully flattened ${file}`);
      } catch (err) {
        console.error(`Error processing ${file}:`, err);
      }
    }
  }
}

// Process en and zh-CN directories
const subDirs = ['en', 'zh-CN'];
subDirs.forEach(subDir => {
  const fullPath = path.join(localesDir, subDir);
  if (fs.existsSync(fullPath)) {
    processDirectory(fullPath);
  } else {
    console.warn(`Directory not found: ${fullPath}`);
  }
});

console.log('Finished flattening i18n files.');
