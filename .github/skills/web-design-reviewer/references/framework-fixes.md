# Framework-specific Fix Guide

This document explains specific fix techniques for each framework and styling method.

---

## Tailwind CSS (used in this project)

### Layout Fixes

```jsx
{/* Before: Overflow */}
<div className="w-full">
  <img src="..." />
</div>

{/* After: Overflow control */}
<div className="w-full max-w-full overflow-hidden">
  <img src="..." className="w-full h-auto object-contain" />
</div>
```

### Text Clipping Prevention

```jsx
{/* Single line truncation */}
<p className="truncate">Long text...</p>

{/* Multi-line truncation */}
<p className="line-clamp-3">Long text...</p>

{/* Allow wrapping */}
<p className="break-words">Long text...</p>
```

### Responsive Support

```jsx
{/* Mobile-first responsive */}
<div className="flex flex-col gap-4 md:flex-row md:gap-6 lg:gap-8">
  <div className="w-full md:w-1/2 lg:w-1/3">Content</div>
</div>
```

### Accessibility Improvements

```jsx
{/* Add focus state */}
<button className="bg-blue-500 text-white hover:bg-blue-600 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2">
  Button
</button>
```

---

## Next.js / App Router (used in this project)

### Global Style Fixes

```css
/* app/globals.css */
html, body {
  max-width: 100vw;
  overflow-x: hidden;
}

img {
  max-width: 100%;
  height: auto;
}
```

### Fixes in Layout Components

```tsx
// app/layout.tsx
export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body className="min-h-screen flex flex-col">
        <header className="sticky top-0 z-50">{/* Header */}</header>
        <main className="flex-1 container mx-auto px-4 py-8">{children}</main>
        <footer>{/* Footer */}</footer>
      </body>
    </html>
  );
}
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
