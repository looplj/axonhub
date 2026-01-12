# Simple Q&A Test Case

This test case demonstrates basic chat completion functionality with simple questions and answers.

## Tests Included

1. **TestSimpleQA** - Tests basic arithmetic question "What is 2 + 2?"
2. **TestSimpleQAWithDifferentModel** - Tests with GPT-3.5-turbo model
3. **TestMultipleQuestions** - Tests multiple questions in sequence

## Features Tested

- Basic chat completion API
- Different model usage (GPT-4o, GPT-3.5-turbo)
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
