import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { get } from "svelte/store";
import { toast } from "../toast";

describe("toast store", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    toast.clear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("starts with empty toasts", () => {
    expect(get(toast)).toEqual([]);
  });

  it("adds a success toast", () => {
    toast.success("Done!");
    const toasts = get(toast);
    expect(toasts).toHaveLength(1);
    expect(toasts[0].type).toBe("success");
    expect(toasts[0].message).toBe("Done!");
  });

  it("adds an error toast", () => {
    toast.error("Failed!");
    const toasts = get(toast);
    expect(toasts).toHaveLength(1);
    expect(toasts[0].type).toBe("error");
    expect(toasts[0].message).toBe("Failed!");
  });

  it("adds an info toast", () => {
    toast.info("FYI");
    const toasts = get(toast);
    expect(toasts).toHaveLength(1);
    expect(toasts[0].type).toBe("info");
  });

  it("adds a warning toast", () => {
    toast.warning("Careful!");
    const toasts = get(toast);
    expect(toasts).toHaveLength(1);
    expect(toasts[0].type).toBe("warning");
  });

  it("auto-dismisses after 5 seconds", () => {
    toast.success("Bye");
    expect(get(toast)).toHaveLength(1);

    vi.advanceTimersByTime(5000);
    expect(get(toast)).toHaveLength(0);
  });

  it("can dismiss a toast manually", () => {
    const id = toast.success("Manual dismiss");
    expect(get(toast)).toHaveLength(1);

    toast.dismiss(id);
    expect(get(toast)).toHaveLength(0);
  });

  it("clears all toasts", () => {
    toast.success("One");
    toast.error("Two");
    toast.info("Three");
    expect(get(toast)).toHaveLength(3);

    toast.clear();
    expect(get(toast)).toHaveLength(0);
  });

  it("supports multiple concurrent toasts", () => {
    toast.success("First");
    toast.error("Second");
    expect(get(toast)).toHaveLength(2);
    expect(get(toast)[0].type).toBe("success");
    expect(get(toast)[1].type).toBe("error");
  });

  it("each toast has a unique id", () => {
    toast.success("A");
    toast.success("B");
    const toasts = get(toast);
    expect(toasts[0].id).not.toBe(toasts[1].id);
  });
});
