import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { API_BASE_URL, ApiError } from "./api";
import {
  groupFAQs,
  listFAQs,
  listPosts,
  listTestimonials,
  getPost,
  getReviewSummary,
  searchFAQs,
  submitEnquiry,
  type FAQ,
} from "./content";

const fetchMock = vi.fn();

function jsonResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response;
}

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function faq(overrides: Partial<FAQ> = {}): FAQ {
  return {
    id: "faq-1",
    question: "Do you offer prenatal massage?",
    answer: "Yes, from the second trimester.",
    sortOrder: 1,
    ...overrides,
  };
}

describe("listPosts", () => {
  it("fetches the live feed and passes filters through", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: [] }));

    await listPosts({ category: "wellness", tag: "massage" });

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${API_BASE_URL}/v1/content/posts?category=wellness&tag=massage`);
    // Content is published instantly from the dashboard, so a cached
    // render would show the wrong thing.
    expect(init.cache).toBe("no-store");
  });

  it("omits the query string when there are no filters", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: [] }));

    await listPosts();

    expect(fetchMock.mock.calls[0][0]).toBe(`${API_BASE_URL}/v1/content/posts`);
  });
});

describe("getPost", () => {
  it("encodes the slug", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { post: { slug: "a b" } }));

    await getPost("a b");

    expect(fetchMock.mock.calls[0][0]).toBe(`${API_BASE_URL}/v1/content/posts/a%20b`);
  });

  it("propagates the 404 a draft produces", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(404, { error: { code: "post_not_found", message: "post not found" } }),
    );

    await expect(getPost("draft")).rejects.toBeInstanceOf(ApiError);
  });
});

describe("listFAQs and listTestimonials", () => {
  it("reads the public routes", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: [faq()] }));
    const faqs = await listFAQs();
    expect(faqs).toHaveLength(1);
    expect(fetchMock.mock.calls[0][0]).toBe(`${API_BASE_URL}/v1/content/faqs`);

    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: [] }));
    await listTestimonials();
    expect(fetchMock.mock.calls[1][0]).toBe(`${API_BASE_URL}/v1/content/testimonials`);
  });

  it("passes a category filter", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: [] }));
    await listFAQs("Booking & payment");
    expect(fetchMock.mock.calls[0][0]).toBe(
      `${API_BASE_URL}/v1/content/faqs?category=Booking%20%26%20payment`,
    );
  });
});

describe("getReviewSummary", () => {
  it("returns the aggregate", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(200, { count: 12, average: 4.8, distribution: { "5": 10, "4": 2 } }),
    );

    const summary = await getReviewSummary();

    expect(summary.count).toBe(12);
    expect(summary.average).toBe(4.8);
  });
});

describe("submitEnquiry", () => {
  it("posts the form", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(201, { received: true }));

    await submitEnquiry({
      name: "Ama Serwaa",
      email: "ama@example.com",
      message: "Do you offer prenatal massage?",
    });

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${API_BASE_URL}/v1/enquiries`);
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toMatchObject({ name: "Ama Serwaa" });
  });

  it("surfaces the rate limit so the form can say so", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(429, { error: { code: "rate_limited", message: "too many requests" } }),
    );

    await expect(
      submitEnquiry({ name: "A", email: "a@example.com", message: "hi" }),
    ).rejects.toMatchObject({ status: 429, code: "rate_limited" });
  });
});

describe("groupFAQs", () => {
  it("groups by category and sorts the uncategorised group last", () => {
    const groups = groupFAQs([
      faq({ id: "1", category: "Booking" }),
      faq({ id: "2" }),
      faq({ id: "3", category: "Booking" }),
      faq({ id: "4", category: "Sessions" }),
    ]);

    expect(groups.map(([name]) => name)).toEqual(["Booking", "Sessions", "More questions"]);
    expect(groups[0][1]).toHaveLength(2);
    expect(groups[2][1]).toHaveLength(1);
  });

  it("treats a blank category as uncategorised", () => {
    const groups = groupFAQs([faq({ category: "   " })]);
    expect(groups[0][0]).toBe("More questions");
  });
});

describe("searchFAQs", () => {
  const faqs = [
    faq({ id: "1", question: "Do you offer prénatal massage?", answer: "Yes." }),
    faq({ id: "2", question: "How do I pay?", answer: "Card or mobile money." }),
    faq({ id: "3", question: "Where are you?", answer: "Online.", category: "Sessions" }),
  ];

  it("matches the question, the answer, and the category", () => {
    expect(searchFAQs(faqs, "massage").map((f) => f.id)).toEqual(["1"]);
    expect(searchFAQs(faqs, "mobile money").map((f) => f.id)).toEqual(["2"]);
    expect(searchFAQs(faqs, "sessions").map((f) => f.id)).toEqual(["3"]);
  });

  it("ignores case and accents, so 'prenatal' finds 'prénatal'", () => {
    expect(searchFAQs(faqs, "PRENATAL").map((f) => f.id)).toEqual(["1"]);
    expect(searchFAQs(faqs, "prénatal").map((f) => f.id)).toEqual(["1"]);
  });

  it("returns everything for a blank query", () => {
    expect(searchFAQs(faqs, "")).toHaveLength(3);
    expect(searchFAQs(faqs, "   ")).toHaveLength(3);
  });

  it("returns nothing rather than everything when there is no match", () => {
    expect(searchFAQs(faqs, "helicopter")).toHaveLength(0);
  });
});
