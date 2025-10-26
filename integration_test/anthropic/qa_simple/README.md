# Simple Q&A Tests

This directory contains basic question and answer tests for the Anthropic SDK, demonstrating simple chat completion functionality.

## Test Cases

### TestSimpleQA
Tests basic Q&A functionality:
- Simple question answering
- Response validation
- Context handling

### TestMultipleQuestions
Tests multiple questions in sequence:
- Sequential question handling
- Response consistency
- Error handling

## Running Tests

```bash
# Run Q&A tests
go test -v .

# Run specific test
go test -v -run TestSimpleQA
```
