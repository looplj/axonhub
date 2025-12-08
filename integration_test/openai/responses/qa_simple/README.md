# Simple Q&A Test Case - Responses API

This test case demonstrates basic response generation functionality using the OpenAI Responses API with simple questions and answers.

## Tests Included

1. **TestSimpleQA** - Tests basic arithmetic question "What is 2 + 2?"
2. **TestSimpleQAWithDifferentQuestion** - Tests with a different question
3. **TestMultipleQuestions** - Tests multiple questions in sequence

## Features Tested

- Basic Responses API usage
- Simple text input/output
- Response validation
- Multiple sequential requests

## Running the Tests

```bash
# Run all tests in this directory
go test -v

# Run a specific test
go test -v -run TestSimpleQA

# Run with verbose output
go test -v -run TestMultipleQuestions
```

## Expected Behavior

1. **Simple Questions**: API should return relevant answers to basic questions
2. **Response Structure**: All responses should have valid output text
3. **Sequential Requests**: Multiple requests should work independently
