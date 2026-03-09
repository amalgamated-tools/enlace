---
name: refactor
description: 'Surgical code refactoring to improve maintainability without changing behavior. Covers extracting functions, renaming variables, breaking down god functions, improving type safety, eliminating code smells, and applying design patterns. Less drastic than repo-rebuilder; use for gradual improvements.'
license: MIT
---

# Refactor

## Overview

Improve code structure and readability without changing external behavior. Refactoring is gradual evolution, not revolution. Use this for improving existing code, not rewriting from scratch.

## When to Use

Use this skill when:

- Code is hard to understand or maintain
- Functions/classes are too large
- Code smells need addressing
- Adding features is difficult due to code structure
- User asks "clean up this code", "refactor this", "improve this"

---

## Refactoring Principles

### The Golden Rules

1. **Behavior is preserved** - Refactoring doesn't change what the code does, only how
2. **Small steps** - Make tiny changes, test after each
3. **Version control is your friend** - Commit before and after each safe state
4. **Tests are essential** - Without tests, you're not refactoring, you're editing
5. **One thing at a time** - Don't mix refactoring with feature changes

### When NOT to Refactor

```
- Code that works and won't change again (if it ain't broke...)
- Critical production code without tests (add tests first)
- When you're under a tight deadline
- "Just because" - need a clear purpose
```

---

## Common Code Smells & Fixes

### 1. Long Method/Function

Break 200-line functions into focused, single-purpose functions.

### 2. Duplicated Code

Extract common logic into shared utilities.

### 3. Large Class/Module (God Object)

Split into classes with single responsibility.

### 4. Long Parameter List

Group related parameters into objects/interfaces.

### 5. Magic Numbers/Strings

```diff
- if (user.status === 2) { /* ... */ }
- setTimeout(callback, 86400000);

+ const UserStatus = { ACTIVE: 1, INACTIVE: 2, SUSPENDED: 3 } as const;
+ const ONE_DAY_MS = 24 * 60 * 60 * 1000;
+ if (user.status === UserStatus.INACTIVE) { /* ... */ }
+ setTimeout(callback, ONE_DAY_MS);
```

### 6. Nested Conditionals — Use Guard Clauses

```diff
- function process(order) {
-   if (order) {
-     if (order.user) {
-       if (order.user.isActive) {
-         return processOrder(order);
-       }
-     }
-   }
- }

+ function process(order) {
+   if (!order) return { error: 'No order' };
+   if (!order.user) return { error: 'No user' };
+   if (!order.user.isActive) return { error: 'User inactive' };
+   return processOrder(order);
+ }
```

### 7. Dead Code

Remove unused functions, imports, and commented-out code. Git history preserves it.

### 8. Feature Envy

Move logic to the object that owns the data.

### 9. Inappropriate Intimacy

Use encapsulation — ask objects for what you need, don't reach into their internals.

---

## Introducing Type Safety (TypeScript)

```diff
- function calculateDiscount(user, total, membership, date) { /* ... */ }

+ type Membership = 'bronze' | 'silver' | 'gold';
+ interface User { id: string; name: string; membership: Membership; }
+ interface DiscountResult { original: number; discount: number; final: number; rate: number; }
+
+ function calculateDiscount(user: User, total: number, date: Date = new Date()): DiscountResult {
+   // ...
+ }
```

---

## Safe Refactoring Process

```
1. PREPARE   — Ensure tests exist; commit current state
2. IDENTIFY  — Find code smell; understand what the code does; plan
3. REFACTOR  — One small change → run tests → commit if passing → repeat
4. VERIFY    — All tests pass; performance unchanged or improved
5. CLEAN UP  — Update comments and documentation; final commit
```

---

## Refactoring Checklist

### Code Quality

- [ ] Functions are small (< 50 lines)
- [ ] Functions do one thing
- [ ] No duplicated code
- [ ] Descriptive names (variables, functions, classes)
- [ ] No magic numbers/strings
- [ ] Dead code removed

### Structure

- [ ] Related code is together
- [ ] Clear module boundaries
- [ ] Dependencies flow in one direction
- [ ] No circular dependencies

### Type Safety

- [ ] Types defined for all public APIs
- [ ] No `any` types without justification
- [ ] Nullable types explicitly marked

### Testing

- [ ] Refactored code is tested
- [ ] Tests cover edge cases
- [ ] All tests pass

---

## Common Refactoring Operations

| Operation | Description |
|-----------|-------------|
| Extract Method | Turn code fragment into method |
| Extract Class | Move behavior to new class |
| Extract Interface | Create interface from implementation |
| Inline Method | Move method body back to caller |
| Rename Method/Variable | Improve clarity |
| Introduce Parameter Object | Group related parameters |
| Replace Conditional with Polymorphism | Use polymorphism instead of switch/if |
| Replace Magic Number with Constant | Named constants |
| Replace Nested Conditional with Guard Clauses | Early returns |
| Introduce Null Object | Eliminate null checks |
| Replace Inheritance with Delegation | Composition over inheritance |
