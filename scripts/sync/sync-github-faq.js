#!/usr/bin/env node

const https = require('https');
const fs = require('fs');
const path = require('path');

// --- Configuration ---
const GITHUB_REPO = process.env.GITHUB_REPO || 'looplj/axonhub';
const GITHUB_TOKEN = process.env.GITHUB_TOKEN;
const AXONHUB_BASE_URL = process.env.AXONHUB_BASE_URL || 'http://localhost:8090/v1';
const AXONHUB_API_KEY = process.env.AXONHUB_API_KEY;
const AXONHUB_MODEL = process.env.AXONHUB_MODEL || 'deepseek-chat';

const STATE_FILE = path.join(__dirname, '.github_faq_state.json');
const DOCS_DIR = path.join(__dirname, '../../docs');
const EN_FAQ_PATH = path.join(DOCS_DIR, 'en/faq.md');
const ZH_FAQ_PATH = path.join(DOCS_DIR, 'zh/faq.md');

// --- Helpers ---

function request(url, options = {}, body = null, retries = 3) {
  return new Promise((resolve, reject) => {
    const executeRequest = (attempt) => {
      const isHttps = url.startsWith('https');
      const client = isHttps ? https : require('http');
      
      const req = client.request(url, options, (res) => {
        let data = '';
        res.on('data', (chunk) => data += chunk);
        res.on('end', () => {
          if (res.statusCode >= 200 && res.statusCode < 300) {
            try {
              resolve(data ? JSON.parse(data) : {});
            } catch (e) {
              resolve(data);
            }
          } else {
            const error = new Error(`Request failed with status ${res.statusCode}: ${data}`);
            if (attempt < retries) {
              const delay = Math.pow(2, attempt) * 1000;
              console.warn(`Request failed (status ${res.statusCode}). Retrying in ${delay}ms... (Attempt ${attempt + 1}/${retries})`);
              setTimeout(() => executeRequest(attempt + 1), delay);
            } else {
              reject(error);
            }
          }
        });
      });

      req.on('error', (error) => {
        if (attempt < retries) {
          const delay = Math.pow(2, attempt) * 1000;
          console.warn(`Request error: ${error.message}. Retrying in ${delay}ms... (Attempt ${attempt + 1}/${retries})`);
          setTimeout(() => executeRequest(attempt + 1), delay);
        } else {
          reject(error);
        }
      });

      if (body) {
        req.write(typeof body === 'string' ? body : JSON.stringify(body));
      }
      req.end();
    };

    executeRequest(0);
  });
}

async function fetchGithubIssues(since) {
  let url = `https://api.github.com/repos/${GITHUB_REPO}/issues?state=all&per_page=100`;
  if (since && since !== '1970-01-01T00:00:00Z') {
    url += `&since=${since}`;
  }
  const options = {
    method: 'GET',
    headers: {
      'User-Agent': 'AxonHub-FAQ-Sync',
      'Accept': 'application/vnd.github.v3+json',
      ...(GITHUB_TOKEN ? { 'Authorization': `token ${GITHUB_TOKEN}` } : {})
    }
  };
  return request(url, options);
}

async function fetchIssueComments(issueNumber) {
  const url = `https://api.github.com/repos/${GITHUB_REPO}/issues/${issueNumber}/comments`;
  const options = {
    method: 'GET',
    headers: {
      'User-Agent': 'AxonHub-FAQ-Sync',
      'Accept': 'application/vnd.github.v3+json',
      ...(GITHUB_TOKEN ? { 'Authorization': `token ${GITHUB_TOKEN}` } : {})
    }
  };
  return request(url, options);
}

async function analyzeIssue(issue, comments, existingFaqs) {
  if (!AXONHUB_API_KEY) {
    throw new Error('AXONHUB_API_KEY is not set');
  }

  const content = `
Title: ${issue.title}
Body: ${issue.body}
Comments:
${comments.map(c => `- ${c.body}`).join('\n')}
  `.trim();

  const prompt = `
You are a technical documentation assistant for AxonHub.
AxonHub is an all-in-one AI development platform that serves as a unified API gateway for multiple AI providers.

First, classify the following GitHub issue into one of these categories:
- feature: A request for a new feature or enhancement.
- bug: A report of a bug or unexpected behavior.
- question: A question about how to use AxonHub or technical clarification.

If the category is "question", determine if it contains a common question and its corresponding clear answer that should be added to the FAQ.
Check if the question is already covered in the existing FAQs provided below. If it is already covered or redundant, set "is_candidate" to false.

Existing FAQs:
${existingFaqs || 'None'}

If it is a "question" and a good candidate (not already covered), extract the Question and Answer in both English and Chinese.
The question should be concise, and the answer should be helpful and accurate based on the discussion.

Return ONLY a JSON object with the following structure:
{
  "category": "feature" | "bug" | "question",
  "is_candidate": boolean,
  "en": { "question": "string", "answer": "string" },
  "zh": { "question": "string", "answer": "string" }
}

Issue Content:
${content}
  `.trim();

  const url = `${AXONHUB_BASE_URL}/chat/completions`;
  const options = {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${AXONHUB_API_KEY}`
    }
  };
  const body = {
    model: AXONHUB_MODEL,
    messages: [
      { role: 'system', content: 'You are a helpful assistant that outputs JSON.' },
      { role: 'user', content: prompt }
    ],
    response_format: { type: 'json_object' }
  };

  const response = await request(url, options, body);
  return JSON.parse(response.choices[0].message.content);
}

function updateFaqFile(filePath, qa, lang) {
  // Ensure directory exists
  const dir = path.dirname(filePath);
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }

  let content = '';
  if (fs.existsSync(filePath)) {
    content = fs.readFileSync(filePath, 'utf8');
  } else {
    content = lang === 'en' ? '# FAQ\n\n' : '# 常见问题\n\n';
  }

  // Check for duplicates in existing content (simple string check)
  if (content.includes(qa.question)) {
    console.log(`Skipping duplicate FAQ in ${lang}: ${qa.question}`);
    return false;
  }

  const newEntry = `## ${qa.question}\n\n${qa.answer}\n\n`;
  content += newEntry;
  fs.writeFileSync(filePath, content, 'utf8');
  return true;
}

function getExistingFaqContent() {
  let enFaq = '';
  let zhFaq = '';
  if (fs.existsSync(EN_FAQ_PATH)) {
    enFaq = fs.readFileSync(EN_FAQ_PATH, 'utf8');
  }
  if (fs.existsSync(ZH_FAQ_PATH)) {
    zhFaq = fs.readFileSync(ZH_FAQ_PATH, 'utf8');
  }
  return `English FAQ:\n${enFaq}\n\nChinese FAQ:\n${zhFaq}`;
}

function saveState(state) {
  fs.writeFileSync(STATE_FILE, JSON.stringify(state, null, 2));
}

// --- Main ---

async function main() {
  if (!AXONHUB_API_KEY) {
    console.error('Error: AXONHUB_API_KEY environment variable is not set.');
    console.log('Please set it using: export AXONHUB_API_KEY=your_key');
    process.exit(1);
  }

  try {
    let state = { last_check: '1970-01-01T00:00:00Z', processed_issues: [] };
    if (fs.existsSync(STATE_FILE)) {
      state = JSON.parse(fs.readFileSync(STATE_FILE, 'utf8'));
    }

    console.log(`Checking issues for ${GITHUB_REPO} since ${state.last_check}...`);
    const issues = await fetchGithubIssues(state.last_check);
    console.log(`Found ${issues.length} updated issues.`);

    // Sort issues by updated_at ascending to process in order
    issues.sort((a, b) => new Date(a.updated_at) - new Date(b.updated_at));

    for (const issue of issues) {
      try {
        // Skip pull requests
        if (issue.pull_request) continue;
        
        // Skip already processed issues
        if (state.processed_issues.includes(issue.id)) continue;

        console.log(`\n--- Processing issue #${issue.number}: ${issue.title} ---`);
        
        const comments = await fetchIssueComments(issue.number);
        const existingFaqs = getExistingFaqContent();
        const analysis = await analyzeIssue(issue, comments, existingFaqs);

        console.log(`Category: ${analysis.category}`);

        if (analysis.category === 'question' && analysis.is_candidate) {
          console.log(`Adding issue #${issue.number} to FAQ.`);
          const addedEn = updateFaqFile(EN_FAQ_PATH, analysis.en, 'en');
          const addedZh = updateFaqFile(ZH_FAQ_PATH, analysis.zh, 'zh');
          
          if (!addedEn && !addedZh) {
            console.log(`Issue #${issue.number} was already in FAQ (duplicate detection).`);
          }
        } else if (analysis.category !== 'question') {
          console.log(`Skipping issue #${issue.number} (Category: ${analysis.category})`);
        } else {
          console.log(`Issue #${issue.number} is a question but not a good FAQ candidate or already covered.`);
        }

        // Update and save state after EACH issue
        state.processed_issues.push(issue.id);
        if (new Date(issue.updated_at) > new Date(state.last_check)) {
          state.last_check = issue.updated_at;
        }
        saveState(state);
        console.log(`State updated for issue #${issue.number}.`);
      } catch (issueError) {
        console.error(`Error processing issue #${issue.number}:`, issueError.message);
        console.log('Continuing with next issue...');
      }
    }

    console.log('\nSync completed successfully!');
  } catch (error) {
    console.error('Error during sync:', error.message);
    process.exit(1);
  }
}

main();
