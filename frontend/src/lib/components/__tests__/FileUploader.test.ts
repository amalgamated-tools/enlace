import { fireEvent, render } from "@testing-library/svelte";
import { describe, it, expect, vi, afterEach } from "vitest";

import FileUploader from "../FileUploader.svelte";

describe("FileUploader", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("dispatches files on drop and resets dragover styling", async () => {
    const { component, container } = render(FileUploader);
    const onFiles = vi.fn();
    component.$on("files", (event) => onFiles(event.detail.files));

    const dropZone = container.querySelector('[role="button"]') as HTMLElement;

    const dataTransfer = new DataTransfer();
    const file = new File(["hello"], "hello.txt", { type: "text/plain" });
    dataTransfer.items.add(file);

    await fireEvent.dragOver(dropZone);
    expect(dropZone.className).toContain("border-accent");

    await fireEvent.drop(dropZone, { dataTransfer });

    expect(onFiles).toHaveBeenCalledTimes(1);
    expect(onFiles).toHaveBeenCalledWith([file]);
    expect(dropZone.className).not.toContain("border-accent");
  });

  it("activates the file input via keyboard or browse button", async () => {
    const { container } = render(FileUploader, { props: { multiple: false } });
    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    const dropZone = container.querySelector('[role="button"]') as HTMLElement;
    const browseButton = container.querySelector('button[type="button"]') as HTMLButtonElement;

    const clickSpy = vi.spyOn(input, "click");

    await fireEvent.keyDown(dropZone, { key: "Enter" });
    await fireEvent.keyDown(dropZone, { key: " " });
    expect(clickSpy).toHaveBeenCalledTimes(2);

    clickSpy.mockClear();
    await fireEvent.click(browseButton);
    expect(clickSpy).toHaveBeenCalledTimes(1);
  });

  it("dispatches files when input selection changes", async () => {
    const { component, container } = render(FileUploader);
    const onFiles = vi.fn();
    component.$on("files", (event) => onFiles(event.detail.files));

    const input = container.querySelector('input[type="file"]') as HTMLInputElement;

    const dataTransfer = new DataTransfer();
    const file = new File(["content"], "manual.txt", { type: "text/plain" });
    dataTransfer.items.add(file);

    await fireEvent.change(input, { target: { files: dataTransfer.files } });

    expect(onFiles).toHaveBeenCalledTimes(1);
    expect(onFiles).toHaveBeenCalledWith([file]);
  });
});
