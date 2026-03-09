# Framework-specific Fix Guide

This document explains specific fix techniques for each framework and styling method.

---

## Svelte + Tailwind CSS v4 (used in this project)

### Layout Fixes

```svelte
<!-- Before: Overflow -->
<div class="w-full">
  <img src="..." />
</div>

<!-- After: Overflow control -->
<div class="w-full max-w-full overflow-hidden">
  <img src="..." class="w-full h-auto object-contain" alt="" />
</div>
```

### Text Clipping Prevention

```svelte
<!-- Single line truncation -->
<p class="truncate">Long text...</p>

<!-- Multi-line truncation -->
<p class="line-clamp-3">Long text...</p>

<!-- Allow wrapping -->
<p class="break-words">Long text...</p>
```

### Responsive Support

```svelte
<!-- Mobile-first responsive -->
<div class="flex flex-col gap-4 md:flex-row md:gap-6 lg:gap-8">
  <div class="w-full md:w-1/2 lg:w-1/3">Content</div>
</div>
```

### Accessibility Improvements

```svelte
<!-- Add focus state -->
<button
  class="bg-blue-500 text-white hover:bg-blue-600 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
>
  Button
</button>
```

---

## Svelte App Structure (this project)

### Global Style Fixes

```css
/* frontend/src/app.css */
html, body {
  max-width: 100vw;
  overflow-x: hidden;
}

img {
  max-width: 100%;
  height: auto;
}
```

### Fixes in Layout Components (`frontend/src/App.svelte`)

```svelte
<div class="min-h-screen bg-surface-subtle">
  <header class="bg-surface border-b border-border sticky top-0 z-50">
    <!-- Header -->
  </header>

  <main class="max-w-6xl mx-auto px-6 py-8">
    <Router {routes} />
  </main>
</div>
```

### Svelte-specific Notes

```svelte
<!-- Use class, not className -->
<button class="px-3 py-2">Save</button>

<!-- Use Svelte event syntax -->
<button on:click={handleSubmit}>Submit</button>

<!-- Reactive declarations for route-aware UI -->
<script lang="ts">
  import { location } from "svelte-spa-router";
  $: isSettings = $location.startsWith("/settings");
</script>
```

---

## Common Patterns

### Fixing Flexbox Layout Issues

```css
.flex-container {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
}

.flex-item {
  flex: 1 1 300px;
  min-width: 0; /* Prevent flexbox overflow */
}
```

### Organizing z-index (relevant: calendar picker)

```css
:root {
  --z-dropdown: 100;
  --z-sticky: 200;
  --z-modal-backdrop: 300;
  --z-modal: 400;
  --z-tooltip: 500;
  --z-popover: 9999; /* portal-rendered */
}
```

### Adding Focus States

```css
button:focus-visible,
a:focus-visible,
input:focus-visible,
textarea:focus-visible {
  outline: 2px solid #2563eb;
  outline-offset: 2px;
}
```

---

## Debugging Techniques

### Detecting Overflow

```javascript
// Run in browser console to detect overflow elements
document.querySelectorAll('*').forEach(el => {
  if (el.scrollWidth > el.clientWidth) {
    console.log('Horizontal overflow:', el);
  }
});
```
