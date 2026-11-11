import { mkdirSync, readFileSync, writeFileSync } from "node:fs";

// mygo runs from this module. Paths below point at the repository root.
// Version comes from the same file the server embeds. Release tags are v*,
// and mygo prefixes that version with tagPrefix when it uploads.
const version = readFileSync("../../internal/build/VERSION", "utf8").trim().replace(/^v/, "");

mkdirSync("../../tmp", { recursive: true });
writeFileSync("../../tmp/desktop-release-notes.md", `## ${version}\n\nAxonHub Desktop ${version}.\n`);

// tagPrefix stays "v" so desktop installers and update manifests upload to
// the same v* GitHub Release as the server. A different prefix would publish
// another tag; with "v", that release is not a second latest, and the
// workflow creates the draft only when it is missing.
export default {
  name: "AxonHub",
  slug: "axonhub-desktop",
  identifier: "com.looplj.axonhub.desktop",
  version,
  main: ".",
  out: "../../build",
  icon: "resources/icon.png",
  // Catalogs in locales/: en.json is the source, zh-Hans.json the translation.
  // `mygo generate` writes locales/catalog_gen.go, and the app's languages
  // are those files.
  i18n: {
    dir: "locales",
    source: "en",
  },
  updates: {
    publicKey: "LYrEj4DGOQLR67inUsimJ/vVDyieh34+x8nQIros2iU=",
    github: "looplj/axonhub",
    tagPrefix: "v",
    changelog: "../../tmp/desktop-release-notes.md",
  },
};
