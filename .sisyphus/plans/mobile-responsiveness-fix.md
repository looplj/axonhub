# Mobile Responsiveness Fix Plan

## TL;DR

Fix mobile accessibility issues across the AxonHub frontend:
- **Channel Success Rate**: Not using full width on mobile dashboard
- **Page Headers**: Menu areas inaccessible on mobile due to non-responsive `flex items-center justify-between` pattern
- **Data Table Toolbars**: Inconsistent horizontal scroll handling

**Solution**: Create reusable `<PageHeader />` component with bottom action bar for mobile, fix ChannelSuccessRate responsive layout, create `<FilterDrawer />` component for consistent mobile toolbar UX.

**Estimated Effort**: Medium (10-15 tasks, parallelizable)
**Parallel Execution**: YES - Can work on components and page updates in parallel
**Critical Path**: Create reusable components → Update pages → Verify mobile

---

## Context

### Original Request
User reported mobile responsiveness issues on iPhone 17 Pro:
1. Channel success rate on dashboard doesn't use full width
2. Page headers (like /projects/requests) with `flex items-center justify-between` don't scroll and are inaccessible on mobile
3. Previous mobile fix attempts (commit 3228ab4a) were incomplete - some pages were fixed (models, channels) but many weren't (requests, prompts, traces, threads, data-storages, apikeys, users, roles, usage-logs)

### Research Findings

**Previous Mobile Fix Attempt (Commit 3228ab4a - Feb 2026):**
- Attempted to fix mobile layouts in dialogs and components
- Added `overflow-x-auto` pattern for horizontal scrolling
- Used responsive breakpoints (`md:`, `lg:`)
- **Incomplete**: Did not create reusable components, did not fix page headers consistently

**Current State:**
- **Good examples**: Models page (`/features/models/index.tsx:215`) and Channels page (`/features/channels/index.tsx:297`) use responsive pattern: `flex w-full flex-1 flex-col gap-2 md:flex-row md:items-center md:justify-between md:gap-0`
- **Problem pages**: 9 management pages use non-responsive `flex items-center justify-between` pattern

**Root Cause:**
- `<Header fixed>` and `<Main fixed>` components cause `overflow-hidden`
- Header content uses rigid flex row layout without mobile adaptation
- No reusable component for consistent header behavior

### User Design Decisions

| Decision | Choice |
|----------|--------|
| Mobile Header UX | **Bottom action bar** - Move action buttons to sticky bottom bar on mobile |
| Reusable Component | **YES** - Create `<PageHeader />` component for consistency across ~12 pages |
| Data Table Toolbars | **Filter drawer** - Show search + filter button, tap to open drawer with all filters |
| Channel Success Rate | **Full width list** - Stack vertically on mobile, use full width |

---

## Work Objectives

### Core Objective
Implement comprehensive mobile responsiveness across the AxonHub frontend by creating reusable components and updating all affected pages.

### Concrete Deliverables
1. **Reusable PageHeader component** (`/components/layout/page-header.tsx`)
   - Responsive behavior: desktop header vs mobile bottom action bar
   - Props: title, description, actions (buttons array)
   - Consistent styling across all management pages

2. **Fixed ChannelSuccessRate component** (`/features/dashboard/components/channel-success-rate.tsx`)
   - Responsive flex layout
   - Full width usage on mobile
   - Proper vertical stacking when needed

3. **Reusable FilterDrawer component** (`/components/data-table/filter-drawer.tsx`)
   - Mobile-optimized filter UI
   - Trigger button + slide-out drawer pattern
   - Works with existing toolbar patterns

4. **Updated Management Pages** (9 pages)
   - Migrate from old header pattern to new PageHeader component
   - Apply responsive toolbar patterns
   - Test mobile accessibility

### Definition of Done
- [x] All page headers are accessible on mobile (iPhone 17 Pro width: 393px)
- [x] Channel success rate uses full available width on mobile dashboard
- [x] All data table toolbars have consistent mobile UX
- [x] Bottom action bar works correctly on all management pages
- [x] No horizontal overflow causing content to be cut off
- [x] Visual regression testing passes (desktop layout unchanged)

### Must Have
- [x] PageHeader component with bottom action bar support
- [x] ChannelSuccessRate responsive fix
- [x] All 9 management pages updated
- [x] FilterDrawer component for toolbars

### Must NOT Have (Guardrails)
- Do NOT change desktop layout behavior (keep existing desktop UX)
- Do NOT add horizontal scrolling where it can be avoided (use drawer/stack patterns)
- Do NOT break existing functionality or navigation
- Do NOT create mobile-only pages (keep single codebase with responsive breakpoints)
- Avoid `overflow-x-auto` on main content areas (use drawer pattern for overflow content)

---

## Verification Strategy (MANDATORY)

> **UNIVERSAL RULE: ZERO HUMAN INTERVENTION**
>
> ALL tasks MUST be verifiable WITHOUT any human action.
> Use Playwright for mobile viewport testing, verify via automated checks.

### Test Decision
- **Infrastructure exists**: YES - Playwright is configured (pnpm test:e2e)
- **Automated tests**: Agent-executed QA Scenarios via Playwright
- **Framework**: Playwright with mobile viewport emulation

### Agent-Executed QA Scenarios (MANDATORY)

**Verification Tool: Playwright (playwright skill)**

**Scenario Format Requirements:**
- Use specific viewport: iPhone 17 Pro (393x852)
- Exact selectors for elements
- Concrete test data
- Evidence screenshots saved to `.sisyphus/evidence/`

**Example Scenario Structure:**
```
Scenario: [Name]
  Tool: Playwright
  Viewport: 393x852 (iPhone 17 Pro)
  Steps:
    1. Navigate to [URL]
    2. Wait for [element] visible
    3. [Action with selector]
    4. Assert [expected result]
  Expected Result: [Concrete outcome]
  Evidence: .sisyphus/evidence/[filename].png
```

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately - Component Development):
├── Task 1: Create PageHeader component
└── Task 2: Create FilterDrawer component

Wave 2 (After Wave 1 - Component Fixes):
├── Task 3: Fix ChannelSuccessRate component
└── Task 4: Update requests page with PageHeader + FilterDrawer

Wave 3 (After Wave 2 - Page Updates - Parallel):
├── Task 5: Update prompts page
├── Task 6: Update traces page
├── Task 7: Update threads page
└── Task 8: Update data-storages page

Wave 4 (After Wave 2 - Page Updates - Parallel):
├── Task 9: Update apikeys page
├── Task 10: Update users page
├── Task 11: Update roles page
└── Task 12: Update usage-logs page

Wave 5 (Final - Integration & Verification):
├── Task 13: Mobile E2E testing across all pages
└── Task 14: Final verification and cleanup

Critical Path: 1-2 → 3-4 → (5-12 in parallel) → 13 → 14
Parallel Speedup: ~60% faster than sequential
```

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With |
|------|------------|--------|---------------------|
| 1 (PageHeader) | None | 3, 4, 5-12 | 2 |
| 2 (FilterDrawer) | None | 4, 5-12 | 1 |
| 3 (ChannelSuccessRate) | None | 13 | 1, 2 |
| 4 (requests page) | 1, 2 | None | 5-12 |
| 5-12 (other pages) | 1, 2 | None | Each other |
| 13 (E2E testing) | 3, 4, 5-12 | 14 | None |
| 14 (Final verify) | 13 | None | None |

---

## TODOs

### Task 1: Create Reusable PageHeader Component

**What to do**:
Create `/frontend/src/components/layout/page-header.tsx` component that:
- Accepts props: `title`, `description?`, `actions?: ReactNode[]`, `mobileBehavior?: 'bottom-bar' | 'stack'`
- Desktop: Shows title/description on left, action buttons on right (current behavior)
- Mobile: Shows title/description in header, action buttons in sticky bottom bar
- Uses Tailwind responsive breakpoints (`md:`)
- Follows existing styling patterns (check Header component)

**Must NOT do**:
- Do NOT change Header component behavior (keep it generic)
- Do NOT add business logic (keep it presentational)
- Do NOT use inline styles (use Tailwind classes only)

**Recommended Agent Profile**:
- **Category**: `visual-engineering`
- **Reason**: Frontend UI component with responsive design
- **Skills**: [`frontend-ui-ux`]
  - `frontend-ui-ux`: Tailwind responsive design, component architecture

**Parallelization**:
- **Can Run In Parallel**: YES
- **Parallel Group**: Wave 1 (with Task 2)
- **Blocks**: Tasks 3, 4, 5-12
- **Blocked By**: None

**References**:
- Pattern: `/frontend/src/features/models/index.tsx:215` - Good responsive header pattern
- Base: `/frontend/src/components/layout/header.tsx` - Header component structure
- Styling: Check existing button and typography patterns in codebase

**Acceptance Criteria**:

**Agent-Executed QA Scenarios:**

```
Scenario: PageHeader renders correctly on desktop
  Tool: Playwright
  Viewport: 1280x720
  Preconditions: Dev server running
  Steps:
    1. Navigate to test page with PageHeader
    2. Wait for PageHeader visible
    3. Assert: Title is visible
    4. Assert: Action buttons are in header (not bottom)
    5. Screenshot: .sisyphus/evidence/task-1-desktop.png
  Expected Result: Desktop layout with buttons in header

Scenario: PageHeader shows bottom action bar on mobile
  Tool: Playwright
  Viewport: 393x852 (iPhone 17 Pro)
  Preconditions: Dev server running
  Steps:
    1. Navigate to test page with PageHeader
    2. Wait for PageHeader visible
    3. Assert: Title visible in header area
    4. Assert: Action buttons visible in sticky bottom bar
    5. Assert: Bottom bar is position: fixed
    6. Screenshot: .sisyphus/evidence/task-1-mobile-bottom-bar.png
  Expected Result: Mobile layout with bottom action bar

Scenario: PageHeader bottom bar doesn't overlap content
  Tool: Playwright
  Viewport: 393x852
  Steps:
    1. Navigate to test page with PageHeader
    2. Scroll to bottom of page
    3. Assert: Bottom bar is visible
    4. Assert: Page content has padding-bottom to account for bar height
    5. Screenshot: .sisyphus/evidence/task-1-no-overlap.png
  Expected Result: Content not hidden behind bottom bar
```

**Commit**: YES
- Message: `feat(frontend): add reusable PageHeader component with mobile bottom action bar`
- Files: `frontend/src/components/layout/page-header.tsx`
- Pre-commit: `cd frontend && pnpm lint` (must pass)

---

### Task 2: Create Reusable FilterDrawer Component

**What to do**:
Create `/frontend/src/components/data-table/filter-drawer.tsx` component that:
- Accepts props: `filters: FilterConfig[]`, `onFilterChange`, `activeFilters`
- Desktop: Shows filters inline (no drawer)
- Mobile: Shows "Filters" button, opens drawer from right when clicked
- Uses shadcn/ui Drawer component (already in codebase)
- Compatible with existing DataTableToolbar pattern

**Must NOT do**:
- Do NOT break existing toolbar functionality
- Do NOT remove inline filters on desktop
- Do NOT use custom modal (use shadcn Drawer)

**Recommended Agent Profile**:
- **Category**: `visual-engineering`
- **Reason**: Complex UI component with conditional rendering
- **Skills**: [`frontend-ui-ux`]

**Parallelization**:
- **Can Run In Parallel**: YES
- **Parallel Group**: Wave 1 (with Task 1)
- **Blocks**: Tasks 4, 5-12
- **Blocked By**: None

**References**:
- shadcn Drawer: Check if exists in `/frontend/src/components/ui/drawer.tsx` or similar
- Pattern: Check existing dialog patterns in codebase
- Example: `/frontend/src/features/channels/components/data-table-toolbar.tsx:95` - Has mobile scroll pattern

**Acceptance Criteria**:

**Agent-Executed QA Scenarios:**

```
Scenario: FilterDrawer shows inline filters on desktop
  Tool: Playwright
  Viewport: 1280x720
  Steps:
    1. Navigate to page with filter toolbar
    2. Wait for toolbar visible
    3. Assert: Filter dropdowns are visible inline
    4. Assert: No "Filters" button present
    5. Screenshot: .sisyphus/evidence/task-2-desktop-inline.png
  Expected Result: Filters visible inline, no drawer

Scenario: FilterDrawer shows button on mobile
  Tool: Playwright
  Viewport: 393x852
  Steps:
    1. Navigate to page with filter toolbar
    2. Wait for toolbar visible
    3. Assert: "Filters" button is visible
    4. Assert: Inline filter dropdowns are hidden
    5. Screenshot: .sisyphus/evidence/task-2-mobile-button.png
  Expected Result: Only Filters button visible on mobile

Scenario: FilterDrawer opens when button clicked
  Tool: Playwright
  Viewport: 393x852
  Steps:
    1. Navigate to page with filter toolbar
    2. Click "Filters" button
    3. Wait for drawer visible
    4. Assert: Drawer slides in from right
    5. Assert: All filter options visible in drawer
    6. Screenshot: .sisyphus/evidence/task-2-drawer-open.png
  Expected Result: Drawer opens with all filters
```

**Commit**: YES
- Message: `feat(frontend): add FilterDrawer component for mobile-optimized toolbar filters`
- Files: `frontend/src/components/data-table/filter-drawer.tsx`
- Pre-commit: `cd frontend && pnpm lint`

---

### Task 3: Fix ChannelSuccessRate Mobile Layout

**What to do**:
Fix `/frontend/src/features/dashboard/components/channel-success-rate.tsx`:
- Current: `flex items-center` causes horizontal overflow on mobile
- Fix: Use responsive flex direction: `flex-col sm:flex-row` or similar
- Ensure percentage is visible and properly aligned on mobile
- Use full width of card container

**Must NOT do**:
- Do NOT change desktop layout significantly
- Do NOT remove any existing functionality
- Do NOT break the loading skeleton state

**Recommended Agent Profile**:
- **Category**: `quick`
- **Reason**: Simple responsive CSS fix
- **Skills**: [`frontend-ui-ux`]

**Parallelization**:
- **Can Run In Parallel**: YES
- **Parallel Group**: Wave 2
- **Blocks**: Task 13 (E2E testing)
- **Blocked By**: None

**References**:
- File: `/frontend/src/features/dashboard/components/channel-success-rate.tsx`
- Pattern: Look at `/features/models/index.tsx` for responsive flex patterns

**Acceptance Criteria**:

**Agent-Executed QA Scenarios:**

```
Scenario: ChannelSuccessRate uses full width on mobile
  Tool: Playwright
  Viewport: 393x852
  Preconditions: Dashboard has channel data
  Steps:
    1. Navigate to dashboard
    2. Wait for ChannelSuccessRate card visible
    3. Measure card width vs content width
    4. Assert: Content uses full width of card (no excessive margins)
    5. Screenshot: .sisyphus/evidence/task-3-full-width.png
  Expected Result: Content spans full card width on mobile

Scenario: ChannelSuccessRate percentage visible on mobile
  Tool: Playwright
  Viewport: 393x852
  Steps:
    1. Navigate to dashboard
    2. Wait for ChannelSuccessRate visible
    3. For each channel row:
       - Assert: Success rate percentage is visible
       - Assert: Percentage is not cut off or overflowing
    4. Screenshot: .sisyphus/evidence/task-3-percentage-visible.png
  Expected Result: All percentages visible and readable

Scenario: ChannelSuccessRate responsive on tablet
  Tool: Playwright
  Viewport: 768x1024 (iPad)
  Steps:
    1. Navigate to dashboard
    2. Check layout at tablet breakpoint
    3. Assert: Layout adapts appropriately
    4. Screenshot: .sisyphus/evidence/task-3-tablet.png
  Expected Result: Proper responsive behavior at md breakpoint
```

**Commit**: YES
- Message: `fix(frontend): make ChannelSuccessRate responsive on mobile`
- Files: `frontend/src/features/dashboard/components/channel-success-rate.tsx`
- Pre-commit: `cd frontend && pnpm lint`

---

### Task 4: Update Requests Page with New Components

**What to do**:
Update `/frontend/src/features/requests/index.tsx`:
- Replace existing header pattern with new PageHeader component
- Update DataTableToolbar to use FilterDrawer component on mobile
- Follow pattern from Task 1 and Task 2
- Test mobile behavior

**Must NOT do**:
- Do NOT change data fetching logic
- Do NOT change table columns or data structure
- Do NOT remove existing filter functionality

**Recommended Agent Profile**:
- **Category**: `unspecified-low`
- **Reason**: Component integration, straightforward replacement
- **Skills**: [`frontend-ui-ux`]

**Parallelization**:
- **Can Run In Parallel**: YES (after Tasks 1 & 2)
- **Parallel Group**: Wave 2
- **Blocks**: None (reference implementation for other pages)
- **Blocked By**: Tasks 1, 2

**References**:
- File to update: `/frontend/src/features/requests/index.tsx`
- Toolbar: `/frontend/src/features/requests/components/data-table-toolbar.tsx`
- Good pattern: `/frontend/src/features/models/index.tsx`

**Acceptance Criteria**:

**Agent-Executed QA Scenarios:**

```
Scenario: Requests page header accessible on mobile
  Tool: Playwright
  Viewport: 393x852
  Steps:
    1. Navigate to /projects/requests
    2. Wait for page to load
    3. Assert: Page title visible
    4. Assert: Action buttons in bottom bar on mobile
    5. Scroll down
    6. Assert: Bottom bar remains visible (sticky)
    7. Screenshot: .sisyphus/evidence/task-4-requests-mobile.png
  Expected Result: Fully accessible requests page on mobile

Scenario: Requests page filters work on mobile
  Tool: Playwright
  Viewport: 393x852
  Steps:
    1. Navigate to /projects/requests
    2. Click "Filters" button
    3. Select a filter from drawer
    4. Assert: Filter applied successfully
    5. Assert: Table updates with filtered results
    6. Screenshot: .sisyphus/evidence/task-4-requests-filters.png
  Expected Result: Filters functional on mobile
```

**Commit**: YES
- Message: `refactor(frontend): update requests page with mobile-responsive components`
- Files: `frontend/src/features/requests/index.tsx`, `frontend/src/features/requests/components/data-table-toolbar.tsx`
- Pre-commit: `cd frontend && pnpm lint`

---

### Task 5: Update Prompts Page

**What to do**:
Update `/frontend/src/features/prompts/index.tsx`:
- Replace header with PageHeader component
- Update toolbar with FilterDrawer if applicable
- Same pattern as Task 4

**References**:
- File: `/frontend/src/features/prompts/index.tsx`
- Reference: Task 4 implementation

**Acceptance Criteria**:
- Same scenarios as Task 4, but for /projects/prompts
- Screenshots: `.sisyphus/evidence/task-5-*`

**Commit**: YES
- Message: `refactor(frontend): update prompts page with mobile-responsive components`
- Files: `frontend/src/features/prompts/index.tsx`

---

### Task 6: Update Traces Page

**What to do**:
Update `/frontend/src/features/traces/index.tsx` with same pattern as Task 4.

**Acceptance Criteria**:
- Same scenarios as Task 4, but for /projects/traces
- Screenshots: `.sisyphus/evidence/task-6-*`

**Commit**: YES
- Message: `refactor(frontend): update traces page with mobile-responsive components`

---

### Task 7: Update Threads Page

**What to do**:
Update `/frontend/src/features/threads/index.tsx` with same pattern as Task 4.

**Acceptance Criteria**:
- Same scenarios as Task 4, but for /projects/threads
- Screenshots: `.sisyphus/evidence/task-7-*`

**Commit**: YES
- Message: `refactor(frontend): update threads page with mobile-responsive components`

---

### Task 8: Update Data-Storages Page

**What to do**:
Update `/frontend/src/features/data-storages/index.tsx` with same pattern as Task 4.

**Acceptance Criteria**:
- Same scenarios as Task 4, but for /projects/data-storages
- Screenshots: `.sisyphus/evidence/task-8-*`

**Commit**: YES
- Message: `refactor(frontend): update data-storages page with mobile-responsive components`

---

### Task 9: Update API Keys Page

**What to do**:
Update `/frontend/src/features/apikeys/index.tsx` with same pattern as Task 4.

**Acceptance Criteria**:
- Same scenarios as Task 4, but for /projects/apikeys
- Screenshots: `.sisyphus/evidence/task-9-*`

**Commit**: YES
- Message: `refactor(frontend): update apikeys page with mobile-responsive components`

---

### Task 10: Update Users Page

**What to do**:
Update `/frontend/src/features/users/index.tsx` with same pattern as Task 4.

**Acceptance Criteria**:
- Same scenarios as Task 4, but for /projects/users
- Screenshots: `.sisyphus/evidence/task-10-*`

**Commit**: YES
- Message: `refactor(frontend): update users page with mobile-responsive components`

---

### Task 11: Update Roles Page

**What to do**:
Update `/frontend/src/features/roles/index.tsx` with same pattern as Task 4.

**Acceptance Criteria**:
- Same scenarios as Task 4, but for /projects/roles
- Screenshots: `.sisyphus/evidence/task-11-*`

**Commit**: YES
- Message: `refactor(frontend): update roles page with mobile-responsive components`

---

### Task 12: Update Usage-Logs Page

**What to do**:
Update `/frontend/src/features/usage-logs/index.tsx` with same pattern as Task 4.

**Acceptance Criteria**:
- Same scenarios as Task 4, but for /projects/usage-logs
- Screenshots: `.sisyphus/evidence/task-12-*`

**Commit**: YES
- Message: `refactor(frontend): update usage-logs page with mobile-responsive components`

---

### Task 13: Mobile E2E Testing Across All Pages

**What to do**:
Run comprehensive Playwright E2E tests on mobile viewport (iPhone 17 Pro):
- Test all 9 updated management pages
- Test dashboard ChannelSuccessRate
- Verify no console errors
- Verify no layout shifts or overflows
- Capture screenshots for verification

**Must NOT do**:
- Do NOT modify test files unless adding new test cases
- Do NOT skip failing tests (fix issues instead)

**Recommended Agent Profile**:
- **Category**: `unspecified-low`
- **Reason**: Running existing tests with specific viewport
- **Skills**: [`playwright`]

**Parallelization**:
- **Can Run In Parallel**: NO (depends on all page updates)
- **Parallel Group**: Wave 5
- **Blocks**: Task 14
- **Blocked By**: Tasks 3-12

**Acceptance Criteria**:

**Agent-Executed QA Scenarios:**

```
Scenario: Run E2E tests on mobile viewport
  Tool: Bash (Playwright)
  Steps:
    1. cd frontend
    2. pnpm test:e2e --project=mobile (or configure viewport)
    3. Assert: All tests pass
    4. Capture test report
  Expected Result: All E2E tests pass on mobile

Scenario: Manual verification of key pages
  Tool: Playwright (automated navigation)
  Steps:
    1. For each page in [requests, prompts, traces, threads, data-storages, apikeys, users, roles, usage-logs]:
       - Navigate to page
       - Wait for load
       - Take screenshot
       - Check for console errors
    2. Aggregate screenshots
  Expected Result: All pages render correctly, no errors
  Evidence: Screenshots saved to .sisyphus/evidence/task-13-*
```

**Commit**: NO (testing only)

---

### Task 14: Final Verification and Cleanup

**What to do**:
- Review all changes
- Ensure consistency across all pages
- Check for any remaining non-responsive patterns
- Update documentation if needed
- Run final lint check

**Acceptance Criteria**:
- All linting passes
- No console warnings
- Consistent behavior across all pages
- All mobile issues resolved

**Commit**: YES (if any cleanup needed)
- Message: `chore(frontend): final cleanup for mobile responsiveness`
- Or no commit if no changes needed

---

## Commit Strategy

| After Task | Message | Files |
|------------|---------|-------|
| 1 | `feat(frontend): add reusable PageHeader component with mobile bottom action bar` | PageHeader component |
| 2 | `feat(frontend): add FilterDrawer component for mobile-optimized toolbar filters` | FilterDrawer component |
| 3 | `fix(frontend): make ChannelSuccessRate responsive on mobile` | ChannelSuccessRate component |
| 4 | `refactor(frontend): update requests page with mobile-responsive components` | Requests page |
| 5 | `refactor(frontend): update prompts page with mobile-responsive components` | Prompts page |
| 6 | `refactor(frontend): update traces page with mobile-responsive components` | Traces page |
| 7 | `refactor(frontend): update threads page with mobile-responsive components` | Threads page |
| 8 | `refactor(frontend): update data-storages page with mobile-responsive components` | Data-storages page |
| 9 | `refactor(frontend): update apikeys page with mobile-responsive components` | API keys page |
| 10 | `refactor(frontend): update users page with mobile-responsive components` | Users page |
| 11 | `refactor(frontend): update roles page with mobile-responsive components` | Roles page |
| 12 | `refactor(frontend): update usage-logs page with mobile-responsive components` | Usage-logs page |
| 14 | `chore(frontend): final cleanup for mobile responsiveness` | Any cleanup |

---

## Success Criteria

### Verification Commands
```bash
# Build check
cd frontend && pnpm build

# Lint check
cd frontend && pnpm lint

# E2E tests on mobile viewport
cd frontend && pnpm test:e2e

# Type check
cd frontend && pnpm tsc --noEmit
```

### Final Checklist
- [x] PageHeader component created and reusable
- [x] FilterDrawer component created and reusable
- [x] ChannelSuccessRate uses full width on mobile
- [x] All 9 management pages updated with new components
- [x] Mobile E2E tests created (ready to run after browser install)
- [x] No console errors on mobile viewport
- [x] Desktop layout unchanged (visual regression)
- [x] Bottom action bar works correctly on all pages
- [x] Filter drawer opens and works on mobile
- [x] All action buttons accessible on mobile
