const { GoogleGenAI } = require("@google/genai");
const { TestConfig } = require("./config");

class StreamingTestHelper {
  constructor() {
    this.config = new TestConfig();

    try {
      this.config.validateConfig();
      this.client = new GoogleGenAI({
        apiKey: this.config.apiKey,
        httpOptions: {
          baseUrl: this.config.baseUrl,
          headers: this.config.getHeaders(),
        },
      });
    } catch (error) {
      console.log(
        `Skipping tests due to configuration error: ${error.message}`
      );
      process.exit(0);
    }
  }

  getModel() {
    return this.config.model;
  }

  createRequestContext() {
    return {
      headers: this.config.getHeaders(),
    };
  }
}

async function collectStream(stream) {
  let fullText = "";
  let chunkCount = 0;

  for await (const chunk of stream) {
    chunkCount++;

    if (!chunk || !chunk.candidates || chunk.candidates.length === 0) {
      continue;
    }

    const candidate = chunk.candidates[0];
    const parts = candidate.content?.parts || [];

    for (const part of parts) {
      if (part?.text) {
        fullText += part.text;
        console.log(`Stream chunk ${chunkCount}: ${part.text}`);
      }
    }
  }

  return { fullText, chunkCount };
}

async function testBasicStreamingChatCompletion() {
  console.log("Running TestBasicStreamingChatCompletion...");

  const helper = new StreamingTestHelper();
  const modelName = helper.getModel();
  const context = helper.createRequestContext();
  const question = "Tell me a short story about a robot learning to paint.";

  console.log(`Sending streaming request: ${question}`);

  try {
    const stream = await helper.client.models.generateContentStream({
      model: modelName,
      contents: [
        {
          role: "user",
          parts: [{ text: question }],
        },
      ],
      ...context,
    });

    const { fullText, chunkCount } = await collectStream(stream);

    console.log(`Total streaming responses: ${chunkCount}`);

    if (!fullText) {
      throw new Error("Expected non-empty streaming response");
    }

    if (
      !fullText.toLowerCase().includes("robot") &&
      !fullText.toLowerCase().includes("paint")
    ) {
      throw new Error(
        `Expected content to mention robot or paint, got: ${fullText}`
      );
    }

    console.log("✅ TestBasicStreamingChatCompletion passed");
  } catch (error) {
    error.stack &&
      console.error("❌ TestBasicStreamingChatCompletion failed:", error.stack);
    console.error("❌ TestBasicStreamingChatCompletion failed:", error.message);
    throw error;
  }
}

async function testLongResponseStreaming() {
  console.log("Running TestLongResponseStreaming...");

  const helper = new StreamingTestHelper();
  const modelName = helper.getModel();
  const context = helper.createRequestContext();
  const question =
    "Write a detailed explanation of how photosynthesis works, including the light-dependent and light-independent reactions.";

  console.log("Sending streaming request for long response...");

  try {
    const stream = await helper.client.models.generateContentStream({
      model: modelName,
      contents: [
        {
          role: "user",
          parts: [{ text: question }],
        },
      ],
      ...context,
    });

    const { fullText, chunkCount } = await collectStream(stream);

    console.log(
      `Long streamed response: ${fullText.length} characters in ${chunkCount} chunks`
    );

    if (fullText.length < 100) {
      throw new Error(
        `Expected longer content, got: ${fullText.length} characters`
      );
    }

    const expectedTerms = [
      "photosynthesis",
      "light",
      "chlorophyll",
      "carbon dioxide",
      "oxygen",
    ];
    const foundTerms = expectedTerms.filter((term) =>
      fullText.toLowerCase().includes(term)
    );

    if (foundTerms.length < 2) {
      throw new Error(
        `Expected explanation to contain more key terms, found ${foundTerms.length}/${expectedTerms.length}`
      );
    }

    console.log("✅ TestLongResponseStreaming passed");
  } catch (error) {
    console.error("❌ TestLongResponseStreaming failed:", error.message);
    throw error;
  }
}

async function runStreamingTests() {
  console.log("🚀 Starting Gemini Node.js Streaming Tests\n");

  const tests = [testBasicStreamingChatCompletion, testLongResponseStreaming];
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
    console.log("");
  }

  console.log(
    `\n📊 Streaming Test Results: ${passed} passed, ${failed} failed`
  );

  if (failed > 0) {
    process.exit(1);
  } else {
    console.log("🎉 All streaming tests passed!");
    process.exit(0);
  }
}

if (require.main === module) {
  runStreamingTests().catch((error) => {
    console.error("❌ Streaming test runner failed:", error.message);
    process.exit(1);
  });
}

module.exports = {
  StreamingTestHelper,
  testBasicStreamingChatCompletion,
  testLongResponseStreaming,
  runStreamingTests,
};
