# Visual Inspection Checklist

A checklist for reviewing the git-stuff-done dashboard UI.

---

## 1. Layout Verification

- [ ] Header is correctly positioned
- [ ] Panels fill available height without overflow
- [ ] Resize handles work between all panels
- [ ] Content does not overflow from panels
- [ ] Calendar popover renders above all panels (portal z-index)

---

## 2. Typography Verification

- [ ] Body text font size is sufficient (minimum 16px)
- [ ] Line height is appropriate (1.5–1.8)
- [ ] Long titles truncate correctly in PR/notification rows
- [ ] Heading size hierarchy is clear

---

## 3. Color & Contrast Verification

- [ ] Body text: Contrast ratio 4.5:1 or higher (AA)
- [ ] Muted text readable against dark backgrounds
- [ ] Dark mode and light mode both pass contrast
- [ ] Green content-indicator dots are visible in both themes
- [ ] Unread notification dot (pink) is visible

---

## 4. Responsive Verification

### Desktop (1280px+)
- [ ] 4-panel layout renders correctly
- [ ] Panel resize handles are functional
- [ ] Hover states (insert button, PR rows) work

### Narrow Desktop / Laptop (1024px)
- [ ] Panels don't collapse unexpectedly
- [ ] Text doesn't clip in constrained panels

---

## 5. Interactive Element Verification

### Editor
- [ ] Tab indents bullet lines
- [ ] Shift+Tab unindents bullet lines
- [ ] Enter auto-continues bullet lists
- [ ] Insert-at-cursor inserts link at correct position

### Calendar Picker
- [ ] Opens on date click
- [ ] Green dots show on days with content
- [ ] Future dates are disabled
- [ ] "Jump to Today" shortcut works
- [ ] Closes on outside click
- [ ] Appears above all panels (not clipped)

### PR & Notification Panels
- [ ] Insert (⌅) button appears on hover at row start
- [ ] Clicking inserts markdown link in editor
- [ ] Refresh button works
- [ ] Polling pauses when tab is hidden

---

## 6. Accessibility Verification

- [ ] All interactive elements accessible via Tab key
- [ ] Calendar day buttons have meaningful states
- [ ] Focus states visible on buttons and links
- [ ] Aria labels present on icon-only buttons (Refresh)

---

## Priority Matrix

| Priority | Category | Examples |
|----------|----------|---------|
| P0 | Functionality breaking | Editor not typing, calendar clipped behind panel |
| P1 | Serious UX issues | Insert button not visible, unreadable text |
| P2 | Moderate issues | Alignment issues, inconsistent spacing |
| P3 | Minor issues | Slight color variations |
