#!/usr/bin/env node

const https = require("node:https");
const http = require("node:http");
const fs = require("node:fs");
const path = require("node:path");
const { URL } = require("node:url");

const SOURCE_URL =
	"https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/refs/heads/dev/dist/all.json";
const CONSTANTS_PATH = path.join(
	__dirname,
	"../../frontend/src/features/models/data/constants.ts",
);
const OUTPUT_PATH = path.join(
	__dirname,
	"../../frontend/src/features/models/data/providers.json",
);
const MODELS_JSON_PATH = path.join(__dirname, "./models.json");

const KWAIPILOT_DEVELOPER_ID = "kwaipilot";

function deepClone(value) {
	return JSON.parse(JSON.stringify(value));
}

function getMetadataItemKey(value) {
	if (value === null) {
		return "null";
	}

	if (Array.isArray(value)) {
		return `array:${JSON.stringify(value)}`;
	}

	if (typeof value === "object") {
		return `object:${JSON.stringify(value)}`;
	}

	return `${typeof value}:${String(value)}`;
}

function isObject(value) {
	return value != null && typeof value === "object" && !Array.isArray(value);
}

function mergeDefined(target, source) {
	if (!isObject(target) || !isObject(source)) {
		return target;
	}

	for (const [key, value] of Object.entries(source)) {
		if (value == null) {
			continue;
		}

		if (Array.isArray(value)) {
			if (!Array.isArray(target[key])) {
				target[key] = deepClone(value);
				continue;
			}

			const merged = [...target[key]];
			const existingKeys = new Set(
				(target[key] || []).map((item) => getMetadataItemKey(item)),
			);

			for (const item of value) {
				const key = getMetadataItemKey(item);
				if (existingKeys.has(key)) {
					continue;
				}

				existingKeys.add(key);
				merged.push(item);
			}

			target[key] = merged;
			continue;
		}

		if (isObject(value)) {
			if (!isObject(target[key])) {
				target[key] = {};
			}
			mergeDefined(target[key], value);
			continue;
		}

		if (target[key] == null || target[key] === "") {
			target[key] = value;
		}
	}

	return target;
}

function normalizeKATSuffix(modelId) {
	if (typeof modelId !== "string") {
		return "";
	}

	const trimmed = modelId.trim();
	if (!trimmed) {
		return "";
	}

	const slashIndex = trimmed.indexOf("/");
	const rawSuffix = slashIndex >= 0 ? trimmed.slice(slashIndex + 1) : trimmed;
	return rawSuffix.trim().toLowerCase();
}

function normalizeKATModelID(modelId) {
	const suffix = normalizeKATSuffix(modelId);
	return suffix;
}

function isKATFamilyModel(model) {
	const modelId = typeof model?.id === "string" ? model.id : "";
	const family = typeof model?.family === "string" ? model.family : "";
	const normalizedFamily = family.toLowerCase();

	return (
		normalizedFamily === "kat-coder" ||
		/^kwaipilot\//i.test(modelId) ||
		/^kuaishou\//i.test(modelId) ||
		/(^|\/)(kat-coder|kat-dev)/i.test(modelId)
	);
}

function buildKWAIPilotProvider(data) {
	const modelsByID = new Map();

	for (const provider of Object.values(data.providers || {})) {
		const models = Array.isArray(provider?.models) ? provider.models : [];
		for (const model of models) {
			if (!isKATFamilyModel(model)) {
				continue;
			}

			const normalizedID = normalizeKATModelID(model.id);
			if (!normalizedID) {
				continue;
			}

			const normalizedModel = deepClone(model);
			normalizedModel.id = normalizedID;
			normalizedModel.family = normalizedModel.family || "kat-coder";

			const existing = modelsByID.get(normalizedID);
			if (existing) {
				mergeDefined(existing, normalizedModel);
			} else {
				modelsByID.set(normalizedID, normalizedModel);
			}
		}
	}

	if (modelsByID.size === 0) {
		return null;
	}

	return {
		id: KWAIPILOT_DEVELOPER_ID,
		name: "KwaiPilot",
		display_name: "KwaiPilot",
		models: Array.from(modelsByID.values()),
	};
}

function fetchJSON(url) {
	return new Promise((resolve, reject) => {
		const proxyUrl =
			process.env.HTTPS_PROXY ||
			process.env.https_proxy ||
			process.env.HTTP_PROXY ||
			process.env.http_proxy;

		if (!proxyUrl) {
			https
				.get(url, (res) => {
					let data = "";

					res.on("data", (chunk) => {
						data += chunk;
					});

					res.on("end", () => {
						try {
							resolve(JSON.parse(data));
						} catch (e) {
							reject(new Error(`Failed to parse JSON: ${e.message}`));
						}
					});
				})
				.on("error", reject);
			return;
		}

		console.log("Using proxy:", proxyUrl);
		const targetUrl = new URL(url);
		const proxy = new URL(proxyUrl);

		const connectOptions = {
			method: "CONNECT",
			host: proxy.hostname,
			port: proxy.port || 80,
			path: `${targetUrl.hostname}:443`,
			headers: { Host: targetUrl.hostname },
		};

		if (proxy.username || proxy.password) {
			const auth = Buffer.from(`${proxy.username}:${proxy.password}`).toString(
				"base64",
			);
			connectOptions.headers["Proxy-Authorization"] = `Basic ${auth}`;
		}

		const connectReq = http.request(connectOptions);

		connectReq.on("connect", (res, socket) => {
			if (res.statusCode !== 200) {
				reject(new Error(`Proxy CONNECT failed: ${res.statusCode}`));
				return;
			}

			const tlsOptions = {
				socket,
				hostname: targetUrl.hostname,
				path: targetUrl.pathname + targetUrl.search,
				method: "GET",
			};

			const tlsReq = https.request(tlsOptions, (tlsRes) => {
				let data = "";
				tlsRes.on("data", (chunk) => {
					data += chunk;
				});
				tlsRes.on("end", () => {
					try {
						resolve(JSON.parse(data));
					} catch (e) {
						reject(new Error(`Failed to parse JSON: ${e.message}`));
					}
				});
			});

			tlsReq.on("error", reject);
			tlsReq.end();
		});

		connectReq.on("error", reject);
		connectReq.end();
	});
}

function extractDeveloperIds(constantsPath) {
	const content = fs.readFileSync(constantsPath, "utf8");
	const match = content.match(/export const DEVELOPER_IDS = \[([\s\S]*?)\]/);

	if (!match) {
		throw new Error("Could not find DEVELOPER_IDS in constants.ts");
	}

	const idsString = match[1];
	const ids = idsString
		.split(",")
		.map((line) => line.trim())
		.filter((line) => line.startsWith("'") || line.startsWith('"'))
		.map((line) => line.replace(/^['"]|['"]$/g, ""));

	return ids;
}

function filterProviders(data, allowedIds) {
	if (!data.providers) {
		throw new Error("Invalid data structure: missing providers field");
	}

	const filtered = {};

	for (const [key, value] of Object.entries(data.providers)) {
		if (allowedIds.includes(value.id)) {
			filtered[key] = value;
		}
	}

	// Map llama channel's llama models to meta developer
	if (allowedIds.includes("meta") && data.providers.llama) {
		const llamaProvider = data.providers.llama;
		const llamaModels = (llamaProvider.models || []).filter((m) =>
			m.id?.toLowerCase().startsWith("llama"),
		);
		if (llamaModels.length > 0) {
			filtered.meta = {
				...llamaProvider,
				id: "meta",
				name: "Meta",
				display_name: "Meta",
				models: llamaModels,
			};
			console.log(
				`Mapped ${llamaModels.length} llama models to meta developer`,
			);
		}
	}

	// Map doubao channel's doubao models to bytedance developer
	if (allowedIds.includes("bytedance") && data.providers.doubao) {
		const doubaoProvider = data.providers.doubao;
		const doubaoModels = (doubaoProvider.models || []).filter((m) =>
			m.id?.toLowerCase().startsWith("doubao"),
		);
		if (doubaoModels.length > 0) {
			filtered.bytedance = {
				...doubaoProvider,
				id: "bytedance",
				name: "ByteDance",
				display_name: "ByteDance",
				models: doubaoModels,
			};
			console.log(
				`Mapped ${doubaoModels.length} doubao models to bytedance developer`,
			);
		}
	}

	if (allowedIds.includes(KWAIPILOT_DEVELOPER_ID)) {
		const kwaipilotProvider = buildKWAIPilotProvider(data);
		if (kwaipilotProvider) {
			filtered[KWAIPILOT_DEVELOPER_ID] = kwaipilotProvider;
			console.log(
				`Mapped ${kwaipilotProvider.models.length} KAT-family models to ${KWAIPILOT_DEVELOPER_ID} developer`,
			);
		}
	}

	if (allowedIds.includes("nvidia") && filtered.nvidia) {
		const nvidiaProvider = filtered.nvidia;
		const nvidiaModels = (nvidiaProvider.models || []).filter((m) =>
			m.id?.toLowerCase().startsWith("nvidia/"),
		);
		if (nvidiaModels.length > 0) {
			filtered.nvidia = {
				...nvidiaProvider,
				models: nvidiaModels,
			};
			console.log(
				`Filtered ${nvidiaModels.length} NVIDIA-created models from ${nvidiaProvider.models.length} total`,
			);
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
		console.log("models.json does not exist, skipping merge");
		return data;
	}

	console.log("Merging with models.json...");
	const modelsJson = JSON.parse(fs.readFileSync(modelsJsonPath, "utf8"));

	for (const [providerKey, models] of Object.entries(modelsJson)) {
		if (data.providers[providerKey]) {
			const existingProvider = data.providers[providerKey];
			if (!existingProvider.models) {
				existingProvider.models = [];
			}

			const existingIds = new Set(existingProvider.models.map((m) => m.id));

			for (const model of models) {
				if (!existingIds.has(model.id)) {
					existingProvider.models.push(model);
					existingIds.add(model.id);
				}
			}
		} else {
			data.providers[providerKey] = {
				id: providerKey,
				models: models,
			};
		}
	}

	return data;
}

async function main() {
	try {
		console.log("Fetching model developers data from:", SOURCE_URL);
		const data = await fetchJSON(SOURCE_URL);

		console.log("Extracting allowed developer IDs from:", CONSTANTS_PATH);
		const allowedIds = extractDeveloperIds(CONSTANTS_PATH);
		console.log("Allowed developer IDs:", allowedIds);

		console.log("Filtering providers...");
		const filtered = filterProviders(data, allowedIds);

		const providerCount = Object.keys(filtered.providers).length;
		console.log(`Filtered to ${providerCount} providers`);

		console.log("Merging with models.json...");
		mergeWithModelsJson(filtered, MODELS_JSON_PATH);

		console.log("Sorting models by release date...");
		sortModelsByDate(filtered);

		console.log("Writing to:", OUTPUT_PATH);
		fs.writeFileSync(OUTPUT_PATH, `${JSON.stringify(filtered, null, 2)}\n`);

		console.log("Sync completed successfully!");
	} catch (error) {
		console.error("Error during sync:", error.message);
		process.exit(1);
	}
}

main();
