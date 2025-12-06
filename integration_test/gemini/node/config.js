const crypto = require('crypto');

class TestConfig {
  constructor() {
    this.apiKey = process.env.TEST_AXONHUB_API_KEY || '';
    this.baseUrl = process.env.TEST_GEMINI_BASE_URL || 'http://localhost:8090/gemini';
    this.model = process.env.TEST_MODEL || 'gemini-2.5-flash';
    this.disableTrace = process.env.TEST_DISABLE_TRACE === 'true';
    this.disableThread = process.env.TEST_DISABLE_THREAD === 'true';
    this.timeout = 30000;
    this.maxRetries = 3;

    if (!this.disableTrace) {
      this.traceId = this.generateRandomId('trace');
    }

    if (!this.disableThread) {
      this.threadId = this.generateRandomId('thread');
    }
  }

  generateRandomId(prefix) {
    const bytes = crypto.randomBytes(8);
    return `${prefix}-${bytes.toString('hex')}`;
  }

  validateConfig() {
    if (!this.apiKey) {
      throw new Error('API key is required (set TEST_AXONHUB_API_KEY environment variable)');
    }

    if (!this.disableTrace && !this.traceId) {
      throw new Error('trace ID is required');
    }

    if (!this.disableThread && !this.threadId) {
      throw new Error('thread ID is required');
    }

    if (!this.model) {
      throw new Error('model is required (set TEST_MODEL environment variable)');
    }
  }

  getHeaders() {
    const headers = {};

    if (!this.disableTrace && this.traceId) {
      headers['AH-Trace-Id'] = this.traceId;
    }

    if (!this.disableThread && this.threadId) {
      headers['AH-Thread-Id'] = this.threadId;
    }

    return headers;
  }
}

module.exports = { TestConfig };
