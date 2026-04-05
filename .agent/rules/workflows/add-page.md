---
description: "Workflow to add a new page with routing, permissions, and sidebar navigation"
---

# Adding a New Page

This workflow describes how to add a new page to the AxonHub frontend with complete routing, permissions, and sidebar integration.

## Overview

To add a new page, you need to create/modify the following files:

1. **Feature Components** - The actual page content
2. **Route File** - TanStack Router file-based route
3. **Sidebar Configuration** - Add navigation item to sidebar
4. **Route Permissions** - Define required permissions
5. **i18n Translations** - Add translation keys

---

## Step-by-Step Guide

### 1. Create Feature Components

Create your feature in the `frontend/src/features/` directory:

```
frontend/src/features/
└── your-feature/
    ├── index.tsx           # Main feature component (exported as default)
    ├── components/         # Feature-specific components
    ├── data/              # Data fetching hooks and queries
    └── types.ts           # TypeScript types
```

**Example `frontend/src/features/your-feature/index.tsx`:**

```tsx
import { useTranslation } from 'react-i18next';

export default function YourFeaturePage() {
  const { t } = useTranslation();

  return (
    <div>
      <h1>{t('yourFeature.title')}</h1>
      {/* Your page content */}
    </div>
  );
}
```

---

### 2. Create Route File

Create a route file in `frontend/src/routes/_authenticated/`:

**For Admin-level pages:**
```
frontend/src/routes/_authenticated/your-feature/index.tsx
```

**For Project-level pages:**
```
frontend/src/routes/_authenticated/project/your-feature/index.tsx
```

**Example route file:**

```tsx
import { createFileRoute } from '@tanstack/react-router';
import YourFeaturePage from '@/features/your-feature';

export const Route = createFileRoute('/_authenticated/your-feature/')({
  component: YourFeaturePage,
});
```

---

### 3. Add Sidebar Navigation

Update `frontend/src/sidebar.ts` to add the navigation item:

**Import the icon:**
```ts
import { IconYourIcon } from '@tabler/icons-react';
```

**Add to the appropriate nav group:**

For Admin pages, add to the "Admin" group:
```ts
{
  title: t('sidebar.items.yourFeature'),
  url: '/your-feature',
  icon: IconYourIcon,
} as NavLink,
```

For Project pages, add to the "Project" group:
```ts
{
  title: t('sidebar.items.yourFeature'),
  url: '/project/your-feature',
  icon: IconYourIcon,
} as NavLink,
```

---

### 4. Configure Route Permissions

Update `frontend/src/config/route-permission.ts` to add permission requirements:

**Add to the appropriate route group:**

```ts
{
  title: 'Admin',  // or 'Project'
  scopeLevel: 'system',  // 'system' for admin, 'any' for project
  routes: [
    // ... existing routes
    {
      path: '/your-feature',
      requiredScopes: ['read_your_feature'],
      mode: 'hidden',
    },
  ],
},
```

**Available scope levels:**
- `'system'` - Only system-level permissions are checked
- `'project'` - Only project-level permissions are checked
- `'any'` - Either system or project permissions grant access

**Mode options:**
- `'hidden'` - Hide the sidebar item when no permission
- `'disabled'` - Show but disable the sidebar item when no permission

---

### 5. Add i18n Translations

Add translation keys to both language files. For feature-specific translations, create dedicated translation files:

**Sidebar item in `frontend/src/locales/en/base.json`:**
```json
{
  "sidebar.items.yourFeature": "Your Feature"
}
```

**Feature translations in `frontend/src/locales/en/yourFeature.json`:**
```json
{
  "yourFeature.title": "Your Feature",
  "yourFeature.description": "Your feature description",
  "yourFeature.actions.create": "Create Item",
  "yourFeature.dialogs.create.title": "Create New Item",
  "yourFeature.messages.createSuccess": "Item created successfully",
  "yourFeature.messages.createError": "Failed to create item: {{error}}"
}
```

**Sidebar item in `frontend/src/locales/zh-CN/base.json`:**
```json
{
  "sidebar.items.yourFeature": "你的功能"
}
```

**Feature translations in `frontend/src/locales/zh-CN/yourFeature.json`:**
```json
{
  "yourFeature.title": "你的功能",
  "yourFeature.description": "你的功能描述",
  "yourFeature.actions.create": "创建项目",
  "yourFeature.dialogs.create.title": "创建新项目",
  "yourFeature.messages.createSuccess": "项目创建成功",
  "yourFeature.messages.createError": "创建项目失败：{{error}}"
}
```

**Translation key naming conventions:**
- `sidebar.items.<feature>` - Sidebar navigation label (in base.json)
- `<feature>.title` - Page title
- `<feature>.description` - Page description
- `<feature>.actions.<action>` - Action button labels
- `<feature>.columns.<column>` - Table column headers
- `<feature>.dialogs.<dialog>.title/description` - Dialog titles and descriptions
- `<feature>.form.<field>.label/placeholder` - Form field labels and placeholders
- `<feature>.messages.<message>` - Success/error messages

---

## Complete Example

Here's a complete example of adding a "Reports" page:

### 1. Create Feature
```tsx
// frontend/src/features/reports/index.tsx
import { useTranslation } from 'react-i18next';

export default function ReportsPage() {
  const { t } = useTranslation();

  return (
    <div className="container mx-auto p-6">
      <h1 className="text-2xl font-bold">{t('reports.title')}</h1>
      <p>{t('reports.description')}</p>
    </div>
  );
}
```

### 2. Create Route
```tsx
// frontend/src/routes/_authenticated/reports/index.tsx
import { createFileRoute } from '@tanstack/react-router';
import ReportsPage from '@/features/reports';

export const Route = createFileRoute('/_authenticated/reports/')({
  component: ReportsPage,
});
```

### 3. Update Sidebar
```ts
// frontend/src/sidebar.ts
import { IconReport } from '@tabler/icons-react';

// In the Admin group:
{
  title: t('sidebar.items.reports'),
  url: '/reports',
  icon: IconReport,
} as NavLink,
```

### 4. Configure Permissions
```ts
// frontend/src/config/route-permission.ts
{
  title: 'Admin',
  scopeLevel: 'system',
  routes: [
    {
      path: '/reports',
      requiredScopes: ['read_reports'],
      mode: 'hidden',
    },
  ],
}
```

### 5. Add Translations
```json
// frontend/src/locales/en/base.json (sidebar item only)
{
  "sidebar.items.reports": "Reports"
}
```

```json
// frontend/src/locales/en/reports.json
{
  "reports.title": "Reports",
  "reports.description": "View system reports",
  "reports.actions.create": "Create Report",
  "reports.messages.createSuccess": "Report created successfully"
}
```

```json
// frontend/src/locales/zh-CN/base.json (sidebar item only)
{
  "sidebar.items.reports": "报表"
}
```

```json
// frontend/src/locales/zh-CN/reports.json
{
  "reports.title": "报表",
  "reports.description": "查看系统报表",
  "reports.actions.create": "创建报表",
  "reports.messages.createSuccess": "报表创建成功"
}
```

---

## File Checklist

- [ ] `frontend/src/features/<feature-name>/index.tsx` - Feature component
- [ ] `frontend/src/routes/_authenticated/<feature-name>/index.tsx` - Route file
- [ ] `frontend/src/sidebar.ts` - Sidebar navigation item
- [ ] `frontend/src/config/route-permission.ts` - Route permission config
- [ ] `frontend/src/locales/en/base.json` - Sidebar item translation only
- [ ] `frontend/src/locales/en/<feature-name>.json` - Feature-specific English translations
- [ ] `frontend/src/locales/zh-CN/base.json` - Sidebar item translation only
- [ ] `frontend/src/locales/zh-CN/<feature-name>.json` - Feature-specific Chinese translations

---

## Notes

1. **Route Generation**: TanStack Router will automatically generate the route tree when you create route files. Run `pnpm dev` to regenerate.

2. **Icons**: Use `@tabler/icons-react` for consistent iconography. Import as: `import { IconName } from '@tabler/icons-react'`.

3. **Permissions**: Backend scope strings typically follow the pattern `read_<resource>` and `write_<resource>`. Ensure these match your backend permission definitions.

4. **Project vs Admin**: 
   - Admin pages are at root level (`/your-feature`)
   - Project pages are nested (`/project/your-feature`)
   - This affects both the route file path and sidebar URL

5. **Default Exports**: Feature index files should export the page component as default export for clean route imports.
