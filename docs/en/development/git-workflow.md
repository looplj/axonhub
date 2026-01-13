# Git Workflow

---

## Development Workflow

1. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make changes and test**
   - Write code
   - Add tests
   - Run tests to ensure they pass
   - Run linter to check code quality

3. **Commit changes**
   ```bash
   git add .
   git commit -m "feat: your feature description"
   ```

4. **Push and create Pull Request**
   ```bash
   git push origin feature/your-feature-name
   ```

## Pre-commit (prek)

This repository includes a `.pre-commit-config.yaml`. `prek` is a drop-in replacement for `pre-commit`.

### Install prek

- macOS/Linux (Homebrew)
  ```bash
  brew install prek
  ```

- Python (uv)
  ```bash
  uv tool install prek
  ```

You can also run it once without installing:

```bash
uvx prek --version
```

- Python (pipx)
  ```bash
  pipx install prek
  ```

- Node.js (pnpm)
  ```bash
  pnpm add -D @j178/prek
  ```

- Standalone installer (Linux/macOS)
  ```bash
  curl --proto '=https' --tlsv1.2 -LsSf https://github.com/j178/prek/releases/latest/download/prek-installer.sh | sh
  ```

If you use the standalone installer, prefer copying the installer URL from the latest GitHub release.

If you're already using `pre-commit` in this repository:
- Replace `pre-commit` commands in your scripts/docs with `prek`.
- Reinstall hooks once with `prek install -f`.

### Run hooks on demand

```bash
prek run
```

Run all hooks against the entire repository:

```bash
prek run --all-files
```

### Install git hooks

```bash
prek install
```

If you previously installed `pre-commit` hooks, reinstall once:

```bash
prek install -f
```

To uninstall:

```bash
prek uninstall
```

If installed via the standalone installer, prek can update itself:

```bash
prek self update
```

## Commit Convention

We follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `style:` Code formatting changes
- `refactor:` Code refactoring
- `test:` Test-related changes
- `chore:` Build process or auxiliary tool changes
