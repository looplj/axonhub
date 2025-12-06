const { GoogleGenAI } = require('@google/genai');
const { TestConfig } = require('./config');

// Test helper class
class TestHelper {
  constructor() {
    this.config = new TestConfig();
    
    try {
      this.config.validateConfig();
      this.client = new GoogleGenAI({
        apiKey: this.config.apiKey,
        httpOptions:{
            baseUrl: this.config.baseUrl,
            headers: this.config.getHeaders()
        }
      });
    } catch (error) {
      console.log(`Skipping tests due to configuration error: ${error.message}`);
      process.exit(0);
    }
  }
  
  printHeaders() {
    console.log(`Using headers: ${JSON.stringify(this.config.getHeaders())}`);
  }
  
  createTestContext() {
    // In Node.js, we'll pass headers through request options
    return {
      headers: this.config.getHeaders()
    };
  }
  
  assertNoError(error, message) {
    if (error) {
      throw new Error(`${message}: ${error.message}`);
    }
  }
  
  validateChatResponse(response, description) {
    if (!response) {
      throw new Error(`Response is null for ${description}`);
    }
    
    if (!response.candidates || response.candidates.length === 0) {
      throw new Error(`No candidates in response for ${description}`);
    }
    
    const candidate = response.candidates[0];
    if (!candidate.content || !candidate.content.parts || candidate.content.parts.length === 0) {
      throw new Error(`Empty content in response for ${description}`);
    }
    
    console.log(`${description} - Response validated successfully: ${response.candidates.length} candidates`);
  }
  
  getModel() {
    return this.config.model;
  }
}

// Utility functions
function containsCaseInsensitive(text, substring) {
  return text.toLowerCase().includes(substring.toLowerCase());
}

function containsAnyCaseInsensitive(text, ...substrings) {
  return substrings.some(substring => containsCaseInsensitive(text, substring));
}

function extractTextFromResponse(response) {
  if (!response || !response.candidates || response.candidates.length === 0) {
    return '';
  }
  
  const candidate = response.candidates[0];
  if (!candidate.content || !candidate.content.parts || candidate.content.parts.length === 0) {
    return '';
  }
  
  return candidate.content.parts
    .filter(part => part.text)
    .map(part => part.text)
    .join('');
}

function containsNumber(text) {
  const numbers = ['4', 'four', 'Four'];
  return numbers.some(num => containsCaseInsensitive(text, num));
}

// Test functions
async function testSimpleQA() {
  console.log('Running TestSimpleQA...');
  
  const helper = new TestHelper();
  helper.printHeaders();
  
  const context = helper.createTestContext();
  const question = 'What is 2 + 2?';
  
  console.log(`Sending question: ${question}`);
  
  const modelName = helper.getModel();
  
  try {
    const response = await helper.client.models.generateContent({
      model: modelName,
      contents: question,
      ...context
    });
    
    helper.validateChatResponse(response, 'Simple Q&A');
    
    const responseText = extractTextFromResponse(response);
    console.log(`Response: ${responseText}`);
    
    if (!containsNumber(responseText)) {
      throw new Error(`Expected response to contain a number, got: ${responseText}`);
    }
    
    console.log('✅ TestSimpleQA passed');
  } catch (error) {
    console.error('❌ TestSimpleQA failed:', error.message);
    throw error;
  }
}

async function testSimpleQAWithDifferentQuestion() {
  console.log('Running TestSimpleQAWithDifferentQuestion...');
  
  const helper = new TestHelper();
  const context = helper.createTestContext();
  const question = 'What is the capital of France?';
  
  console.log(`Sending question: ${question}`);
  
  const modelName = helper.getModel();
  
  try {
    const response = await helper.client.models.generateContent({
      model: modelName,
      contents: question,
      ...context
    });
    
    helper.validateChatResponse(response, 'Simple Q&A with capital question');
    
    const responseText = extractTextFromResponse(response);
    console.log(`Response: ${responseText}`);
    
    if (!containsCaseInsensitive(responseText, 'Paris')) {
      throw new Error(`Expected response to contain 'Paris', got: ${responseText}`);
    }
    
    console.log('✅ TestSimpleQAWithDifferentQuestion passed');
  } catch (error) {
    console.error('❌ TestSimpleQAWithDifferentQuestion failed:', error.message);
    throw error;
  }
}

async function testMultipleQuestions() {
  console.log('Running TestMultipleQuestions...');
  
  const helper = new TestHelper();
  const context = helper.createTestContext();
  
  const questions = [
    'What is the largest planet in our solar system?',
    'Who wrote Romeo and Juliet?',
    'What is the chemical symbol for gold?'
  ];
  
  const modelName = helper.getModel();
  
  try {
    for (let i = 0; i < questions.length; i++) {
      const question = questions[i];
      console.log(`Question ${i + 1}: ${question}`);
      
      const response = await helper.client.models.generateContent({
        model: modelName,
        contents: question,
        ...context
      });
      
      helper.validateChatResponse(response, `Question ${i + 1}`);
      
      const responseText = extractTextFromResponse(response);
      console.log(`Answer ${i + 1}: ${responseText}`);
    }
    
    console.log('✅ TestMultipleQuestions passed');
  } catch (error) {
    console.error('❌ TestMultipleQuestions failed:', error.message);
    throw error;
  }
}

async function testConversationHistory() {
  console.log('Running TestConversationHistory...');
  
  const helper = new TestHelper();
  const context = helper.createTestContext();
  const modelName = helper.getModel();
  
  try {
    // Start a chat session
    const chat = helper.client.chats.create({
      model: modelName,
      history: [],
      config: {
        temperature: 0.5
      }
    });
    
    // First question
    const question1 = "My name is Alice. What's your name?";
    console.log(`Question 1: ${question1}`);
    
    const response1 = await chat.sendMessage(question1);
    helper.validateChatResponse(response1, 'First message');
    
    const responseText1 = extractTextFromResponse(response1);
    console.log(`Response 1: ${responseText1}`);
    
    // Follow-up question that references the previous context
    const question2 = 'What did I just tell you my name is?';
    console.log(`Question 2: ${question2}`);
    
    const response2 = await chat.sendMessage(question2);
    helper.validateChatResponse(response2, 'Second message');
    
    const responseText2 = extractTextFromResponse(response2);
    console.log(`Response 2: ${responseText2}`);
    
    // Verify the model remembers the name
    if (!containsAnyCaseInsensitive(responseText2, 'Alice', 'alice')) {
      throw new Error(`Expected response to contain 'Alice', got: ${responseText2}`);
    }
    
    console.log('✅ TestConversationHistory passed');
  } catch (error) {
    console.error('❌ TestConversationHistory failed:', error.message);
    throw error;
  }
}

// Main test runner
async function runTests() {
  console.log('🚀 Starting Gemini Node.js Integration Tests\n');
  
  const tests = [
    testSimpleQA,
    testSimpleQAWithDifferentQuestion,
    testMultipleQuestions,
    testConversationHistory
  ];
  
  let passed = 0;
  let failed = 0;
  
  for (const test of tests) {
    try {
      await test();
      passed++;
    } catch (error) {
      failed++;
      console.error(`Test failed: ${error.message}`);
    }
    console.log(''); // Empty line for readability
  }
  
  console.log(`\n📊 Test Results: ${passed} passed, ${failed} failed`);
  
  if (failed > 0) {
    process.exit(1);
  } else {
    console.log('🎉 All tests passed!');
    process.exit(0);
  }
}

// Run tests if this file is executed directly
if (require.main === module) {
  runTests().catch(error => {
    console.error('❌ Test runner failed:', error.message);
    process.exit(1);
  });
}

module.exports = {
  TestConfig,
  TestHelper,
  testSimpleQA,
  testSimpleQAWithDifferentQuestion,
  testMultipleQuestions,
  testConversationHistory,
  runTests
};