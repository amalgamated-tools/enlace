import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

vi.mock("svelte-spa-router", () => ({
  push: vi.fn(),
}));

import { push } from "svelte-spa-router";
import ShareCard from "../ShareCard.svelte";

const baseShare = {
  id: "share-1",
  name: "Demo Share",
  description: "A sample description",
  slug: "demo-share",
  view_count: 3,
  download_count: 1,
  created_at: "2024-07-15T12:00:00Z",
  has_password: false,
  is_reverse_share: false,
};

describe("ShareCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders share details and optional badges", () => {
    const customDate = "Custom Date";
    vi.spyOn(Date.prototype, "toLocaleDateString").mockReturnValue(customDate);

    render(ShareCard, {
      props: {
        share: { ...baseShare, has_password: true, is_reverse_share: true },
      },
    });

    expect(screen.getByText("Demo Share")).toBeInTheDocument();
    expect(screen.getByText("A sample description")).toBeInTheDocument();
    expect(screen.getByText("/demo-share")).toBeInTheDocument();
    expect(screen.getByText("3 views")).toBeInTheDocument();
    expect(screen.getByText("1 download")).toBeInTheDocument();
    expect(screen.getByText("Protected")).toBeInTheDocument();
    expect(screen.getByText("Upload")).toBeInTheDocument();
    expect(screen.getByText(customDate)).toBeInTheDocument();
  });

  it("navigates when clicked or activated via keyboard", async () => {
    render(ShareCard, { props: { share: baseShare } });

    const card = screen.getByRole("button", { name: /view share: demo share/i });

    await fireEvent.click(card);
    expect(push).toHaveBeenCalledWith("/shares/share-1");

    vi.clearAllMocks();

    await fireEvent.keyDown(card, { key: "Enter" });
    await fireEvent.keyDown(card, { key: " " });
    expect(push).toHaveBeenCalledTimes(2);
    expect(push).toHaveBeenCalledWith("/shares/share-1");
  });
});
