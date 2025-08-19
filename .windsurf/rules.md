# AxonHub Windsurf Rules

## Project Overview
AxonHub is a Go-based backend with React TypeScript frontend for AI model management and channel operations.

## General Rules

### File Organization
1. **Summary files** MUST be stored in `.trae/summary/` directory
2. **Configuration files** should remain in project root
3. **Frontend assets** belong in `frontend/src/` with proper component organization
4. **Backend code** follows Go project structure in `cmd/`, `internal/`, `conf/`

### Development Workflow
1. **Never restart development servers manually** - they auto-reload on changes
2. **Always test changes** before committing
3. **Follow existing code patterns** and architectural decisions
4. **Use existing dependencies** when possible

## Backend Rules

### Development Environment
1. **Air manages the server** - it rebuilds and restarts automatically on code changes
2. **Never restart the server manually** during development
3. **Use `go build -o axonhub`** to verify builds before committing

### Code Generation
1. **Run `make generate`** after any schema changes (Ent or GraphQL)
2. **Never modify `*.resolvers.go` files directly** - they are auto-generated
3. **Schema changes require regeneration** to update models and resolvers

### Go Code Standards
1. **Use `github.com/samber/lo`** for collection operations (slice, map, ptr utilities)
2. **Follow Go naming conventions** (PascalCase for exported, camelCase for unexported)
3. **Use proper error handling** with wrapped errors
4. **Add proper logging** using the internal log package
5. **Write tests** for new functionality

### Database & Schema
1. **Ent schemas** define database models in `internal/ent/schema/`
2. **GraphQL schemas** define API contracts
3. **Always run `make generate`** after schema modifications
4. **Test database migrations** in development

## Frontend Rules

### Development Environment
1. **Never restart the development server** - it's already running
2. **Use `pnpm`** as the package manager
3. **Run `pnpm dev`** only if server needs to be started fresh

### React/TypeScript Standards
1. **Use TypeScript strictly** - no `any` types without justification
2. **Follow React hooks patterns** for state management
3. **Use existing component patterns** from the codebase
4. **Implement proper error boundaries**
5. **Use the established theme system** with color schemes

### Component Organization
1. **Feature-based structure** in `src/features/`
2. **Shared components** in `src/components/`
3. **Utilities and hooks** in appropriate directories
4. **Assets** in `src/assets/`

### Styling Guidelines
1. **Use Tailwind CSS** for styling
2. **Follow the established theme system** with OKLCH color space
3. **Implement responsive design** with mobile-first approach
4. **Use existing animation patterns** for consistency

## Internationalization (i18n)

### Translation Management
1. **MUST add i18n keys** to `locales/*.json` files for new text
2. **MUST keep keys consistent** between code and JSON files
3. **Use descriptive key names** that indicate context
4. **Support both Chinese (zh) and English (en)**

### Implementation Rules
1. **Use translation hooks** for dynamic content
2. **Implement language switching** where appropriate
3. **Persist language preferences** in localStorage
4. **Validate all translated content**

## Authentication & Testing

### Login Credentials
- **Email**: `my@example.com`
- **Password**: `pwd123456`
- Use these credentials for frontend testing

### Testing Standards
1. **Write unit tests** for business logic
2. **Implement integration tests** for API endpoints
3. **Add E2E tests** for critical user flows
4. **Test i18n functionality** in both languages

## Code Quality

### General Standards
1. **Follow existing code patterns** and conventions
2. **Use meaningful variable and function names**
3. **Add proper documentation** for complex logic
4. **Implement error handling** consistently
5. **Optimize for performance** where needed

### Git Workflow
1. **Use descriptive commit messages**
2. **Keep commits atomic** and focused
3. **Test before committing**
4. **Follow branch naming conventions**

## Dependencies

### Backend Dependencies
1. **Prefer standard library** when possible
2. **Use established packages** like `samber/lo` for utilities
3. **Keep dependencies minimal** and well-maintained

### Frontend Dependencies
1. **Use existing UI libraries** and components
2. **Avoid duplicate functionality**
3. **Consider bundle size** impact
4. **Use TypeScript-compatible packages**

## Performance Guidelines

### Backend Performance
1. **Optimize database queries**
2. **Use proper indexing**
3. **Implement caching** where appropriate
4. **Monitor memory usage**

### Frontend Performance
1. **Implement code splitting**
2. **Optimize bundle size**
3. **Use lazy loading** for components
4. **Minimize re-renders**

## Security Considerations

1. **Never hardcode secrets** or API keys
2. **Use environment variables** for configuration
3. **Implement proper authentication** flows
4. **Validate all inputs** on both frontend and backend
5. **Follow security best practices** for API design

## Deployment

1. **Test locally** before deployment
2. **Use proper build commands**
3. **Verify all dependencies** are included
4. **Check configuration** for production environment
