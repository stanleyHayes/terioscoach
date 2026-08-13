import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ClientSummary } from "@/lib/clients";
import type { FormDefinition, FormField, FormSubmission, SubmissionView } from "@/lib/forms";
import FormsPage from "./page";

const list = vi.hoisted(() => vi.fn());
const create = vi.hoisted(() => vi.fn());
const update = vi.hoisted(() => vi.fn());
const remove = vi.hoisted(() => vi.fn());
const assign = vi.hoisted(() => vi.fn());
const listSubmissions = vi.hoisted(() => vi.fn());
const getSubmission = vi.hoisted(() => vi.fn());
const listClients = vi.hoisted(() => vi.fn());

vi.mock("@/lib/forms", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/forms")>();
  return {
    ...original,
    formsApi: { list, create, update, remove, assign, listSubmissions, getSubmission },
  };
});

vi.mock("@/lib/clients", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/clients")>();
  return { ...original, clientsApi: { ...original.clientsApi, list: listClients } };
});

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  // One frozen value, not a fresh literal per render. The real provider
  // memoizes; a mock that doesn't would re-run every useResource effect on
  // every render and hide exactly the kind of refetch loop worth catching.
  const value = {
    status: "authenticated",
    user: { id: "prac-1", email: "t@example.com", role: "practitioner", name: "Terios" },
    session: { accessToken: "a1", refreshToken: "r1" },
    refreshCallbacks: { onTokensRefreshed: vi.fn() },
    logout: vi.fn(),
  };
  return { ...original, useAuth: () => value };
});

function field(overrides: Partial<FormField> = {}): FormField {
  return { key: "allergies", label: "Any allergies?", type: "text", required: true, options: [], ...overrides };
}

function aForm(overrides: Partial<FormDefinition> = {}): FormDefinition {
  return {
    id: "form-1",
    title: "Health intake",
    description: "So we know what to avoid.",
    fields: [field()],
    template: false,
    sortOrder: 0,
    active: true,
    createdAt: "2026-08-01T09:00:00Z",
    updatedAt: "2026-08-01T09:00:00Z",
    ...overrides,
  };
}

function aSubmission(overrides: Partial<FormSubmission> = {}): FormSubmission {
  return {
    id: "sub-1",
    formId: "form-1",
    formTitle: "Health intake",
    clientId: "client-1",
    status: "submitted",
    answers: { allergies: { value: "None" } },
    signature: { typedName: "Ama Kwarteng", signedAt: "2026-08-04T10:00:00Z" },
    assignedAt: "2026-08-02T09:00:00Z",
    submittedAt: "2026-08-04T10:00:00Z",
    ...overrides,
  };
}

function aClient(overrides: Partial<ClientSummary> = {}): ClientSummary {
  return {
    id: "client-1",
    name: "Ama Kwarteng",
    email: "ama@example.com",
    tags: [],
    totalSessions: 2,
    ...overrides,
  };
}

describe("FormsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    list.mockResolvedValue([aForm()]);
    listSubmissions.mockResolvedValue([aSubmission()]);
    listClients.mockResolvedValue([aClient()]);
    update.mockImplementation((_s, _c, id, patch) => Promise.resolve(aForm({ id, ...patch })));
    create.mockImplementation((_s, _c, draft) => Promise.resolve(aForm({ id: "form-new", ...draft })));
  });

  it("lists forms with how many questions each has", async () => {
    render(<FormsPage />);

    expect(await screen.findByText("Health intake")).toBeTruthy();
    expect(screen.getByText(/1 question · has signed responses/)).toBeTruthy();
  });

  it("turns a form off without deleting it", async () => {
    render(<FormsPage />);
    await screen.findByText("Health intake");

    fireEvent.click(screen.getByRole("switch", { name: /use "health intake"/i }));

    await waitFor(() =>
      expect(update).toHaveBeenCalledWith(expect.anything(), expect.anything(), "form-1", {
        active: false,
      }),
    );
    expect(remove).not.toHaveBeenCalled();
  });

  describe("the builder", () => {
    it("refuses to save a form with no questions", async () => {
      render(<FormsPage />);
      await screen.findByText("Health intake");

      fireEvent.click(screen.getByRole("button", { name: /new form/i }));
      fireEvent.change(screen.getByRole("textbox", { name: /^title/i }), {
        target: { value: "Consent" },
      });
      fireEvent.click(screen.getByRole("button", { name: /create form/i }));

      expect(await screen.findByText(/at least one question/i)).toBeTruthy();
      expect(create).not.toHaveBeenCalled();
    });

    it("holds back a blank question's error until save is tried", async () => {
      render(<FormsPage />);
      await screen.findByText("Health intake");

      fireEvent.click(screen.getByRole("button", { name: /new form/i }));
      fireEvent.click(screen.getByRole("button", { name: /add question/i }));

      // Just added — not a mistake yet.
      expect(screen.queryByText(/every question needs a label/i)).toBeNull();

      fireEvent.click(screen.getByRole("button", { name: /create form/i }));
      expect(await screen.findByText(/every question needs a label/i)).toBeTruthy();
    });

    it("builds a form and sends the trimmed field list", async () => {
      render(<FormsPage />);
      await screen.findByText("Health intake");

      fireEvent.click(screen.getByRole("button", { name: /new form/i }));
      fireEvent.change(screen.getByRole("textbox", { name: /^title/i }), {
        target: { value: "  Consent  " },
      });
      fireEvent.click(screen.getByRole("button", { name: /add question/i }));
      fireEvent.change(screen.getByRole("textbox", { name: /^question 1/i }), {
        target: { value: "  Do you agree?  " },
      });
      fireEvent.click(screen.getByRole("button", { name: /create form/i }));

      await waitFor(() => expect(create).toHaveBeenCalled());
      const draft = create.mock.calls[0]![2];
      expect(draft.title).toBe("Consent");
      expect(draft.fields).toHaveLength(1);
      expect(draft.fields[0].label).toBe("Do you agree?");
      expect(draft.fields[0].type).toBe("text");
    });

    it("uses a custom listbox for the answer type, never a native select", async () => {
      render(<FormsPage />);
      await screen.findByText("Health intake");

      fireEvent.click(screen.getByRole("button", { name: /new form/i }));
      fireEvent.click(screen.getByRole("button", { name: /add question/i }));

      expect(document.querySelectorAll("select").length).toBe(0);
      const combobox = screen.getByRole("combobox", { name: /answer type/i });
      expect(combobox.getAttribute("aria-expanded")).toBe("false");

      fireEvent.click(combobox);
      expect(screen.getByRole("listbox")).toBeTruthy();
      fireEvent.click(screen.getByRole("option", { name: /longer lists of options/i }));

      // Choosing a choice type gives it an options editor to fill in.
      expect(screen.getByRole("textbox", { name: /^option 1/i })).toBeTruthy();
      expect(screen.queryByRole("listbox")).toBeNull();
    });

    it("closes the type listbox on Escape without changing the type", async () => {
      render(<FormsPage />);
      await screen.findByText("Health intake");

      fireEvent.click(screen.getByRole("button", { name: /new form/i }));
      fireEvent.click(screen.getByRole("button", { name: /add question/i }));

      const combobox = screen.getByRole("combobox", { name: /answer type/i });
      fireEvent.keyDown(combobox, { key: "ArrowDown" });
      expect(screen.getByRole("listbox")).toBeTruthy();

      fireEvent.keyDown(combobox, { key: "Escape" });
      expect(screen.queryByRole("listbox")).toBeNull();
      expect(combobox.textContent).toContain("Short answer");
    });

    it("refuses a choice field with no options filled in", async () => {
      render(<FormsPage />);
      await screen.findByText("Health intake");

      fireEvent.click(screen.getByRole("button", { name: /new form/i }));
      fireEvent.change(screen.getByRole("textbox", { name: /^title/i }), {
        target: { value: "Preferences" },
      });
      fireEvent.click(screen.getByRole("button", { name: /add question/i }));
      fireEvent.change(screen.getByRole("textbox", { name: /^question 1/i }), {
        target: { value: "Preferred pressure" },
      });
      fireEvent.click(screen.getByRole("combobox", { name: /answer type/i }));
      fireEvent.click(screen.getByRole("option", { name: /three or four options/i }));

      fireEvent.click(screen.getByRole("button", { name: /create form/i }));

      expect(await screen.findByText(/add at least one option/i)).toBeTruthy();
      expect(create).not.toHaveBeenCalled();
    });

    it("drops options when a choice field is changed back to free text", async () => {
      render(<FormsPage />);
      await screen.findByText("Health intake");

      fireEvent.click(screen.getByRole("button", { name: /new form/i }));
      fireEvent.change(screen.getByRole("textbox", { name: /^title/i }), {
        target: { value: "Preferences" },
      });
      fireEvent.click(screen.getByRole("button", { name: /add question/i }));
      fireEvent.change(screen.getByRole("textbox", { name: /^question 1/i }), {
        target: { value: "Preferred pressure" },
      });
      fireEvent.click(screen.getByRole("combobox", { name: /answer type/i }));
      fireEvent.click(screen.getByRole("option", { name: /three or four options/i }));
      fireEvent.change(screen.getByRole("textbox", { name: /^option 1/i }), {
        target: { value: "Light" },
      });

      fireEvent.click(screen.getByRole("combobox", { name: /answer type/i }));
      fireEvent.click(screen.getByRole("option", { name: /a paragraph or more/i }));
      fireEvent.click(screen.getByRole("button", { name: /create form/i }));

      // The server rejects options on a non-choice field, so they must not
      // survive the switch.
      await waitFor(() => expect(create).toHaveBeenCalled());
      expect(create.mock.calls[0]![2].fields[0].options).toEqual([]);
    });

    it("forces a signature field to be required", async () => {
      render(<FormsPage />);
      await screen.findByText("Health intake");

      fireEvent.click(screen.getByRole("button", { name: /new form/i }));
      fireEvent.click(screen.getByRole("button", { name: /add question/i }));
      fireEvent.click(screen.getByRole("combobox", { name: /answer type/i }));
      fireEvent.click(screen.getByRole("option", { name: /draws and types/i }));

      const toggle = screen.getByRole("switch", { name: /must be answered/i });
      expect(toggle.getAttribute("aria-checked")).toBe("true");
      expect(toggle.hasAttribute("disabled")).toBe(true);
    });

    it("keeps a field's key when its label is reworded", async () => {
      list.mockResolvedValue([aForm({ fields: [field({ key: "allergies", label: "Any allergies?" })] })]);
      render(<FormsPage />);
      await screen.findByText("Health intake");

      fireEvent.click(screen.getByRole("button", { name: /^edit$/i }));
      fireEvent.change(screen.getByRole("textbox", { name: /^question 1/i }), {
        target: { value: "Do you have any allergies at all?" },
      });
      fireEvent.click(screen.getByRole("button", { name: /save changes/i }));

      await waitFor(() => expect(update).toHaveBeenCalled());
      // An answer is stored under the key. Moving it would orphan every
      // answer already recorded.
      expect(update.mock.calls[0]![3].fields[0].key).toBe("allergies");
    });

    it("warns before rewording a form that has signed responses", async () => {
      render(<FormsPage />);
      await screen.findByText("Health intake");

      fireEvent.click(screen.getByRole("button", { name: /^edit$/i }));

      expect(screen.getByText(/keep the wording they were signed under/i)).toBeTruthy();
    });
  });

  describe("assigning", () => {
    it("sends a form to a chosen client", async () => {
      assign.mockResolvedValue(aSubmission({ id: "sub-2", status: "assigned", submittedAt: undefined }));
      render(<FormsPage />);
      await screen.findByText("Health intake");

      fireEvent.click(screen.getByRole("button", { name: /send to a client/i }));
      const dialog = await screen.findByRole("dialog");

      // Nothing is chosen yet, so there is nothing to send.
      expect(within(dialog).getByRole("button", { name: /send it/i }).hasAttribute("disabled")).toBe(true);

      fireEvent.click(await within(dialog).findByRole("radio", { name: /ama kwarteng/i }));
      fireEvent.click(within(dialog).getByRole("button", { name: /send it/i }));

      await waitFor(() =>
        expect(assign).toHaveBeenCalledWith(expect.anything(), expect.anything(), {
          formId: "form-1",
          clientId: "client-1",
        }),
      );
    });

    it("filters the client list in the browser", async () => {
      listClients.mockResolvedValue([
        aClient({ id: "c1", name: "Ama Kwarteng", email: "ama@example.com" }),
        aClient({ id: "c2", name: "Kofi Mensah", email: "kofi@example.com" }),
      ]);
      render(<FormsPage />);
      await screen.findByText("Health intake");

      fireEvent.click(screen.getByRole("button", { name: /send to a client/i }));
      const dialog = await screen.findByRole("dialog");
      await within(dialog).findByRole("radio", { name: /kofi/i });

      fireEvent.change(within(dialog).getByRole("textbox", { name: /find a client/i }), {
        target: { value: "kofi@" },
      });

      expect(within(dialog).queryByRole("radio", { name: /ama/i })).toBeNull();
      expect(within(dialog).getByRole("radio", { name: /kofi/i })).toBeTruthy();
      // One load, no round trip per keystroke.
      expect(listClients).toHaveBeenCalledTimes(1);
    });
  });

  describe("responses", () => {
    async function openResponses() {
      fireEvent.click(screen.getByRole("tab", { name: /responses/i }));
    }

    it("opens on what has been signed", async () => {
      listSubmissions.mockResolvedValue([
        aSubmission({ id: "s1", status: "submitted" }),
        aSubmission({ id: "s2", status: "assigned", submittedAt: undefined }),
      ]);
      render(<FormsPage />);
      await screen.findByText("Health intake");
      await openResponses();

      expect(await screen.findAllByText("Signed")).toBeTruthy();
      expect(screen.getByText(/1 still waiting on a client/i)).toBeTruthy();
      expect(screen.queryByText("Waiting", { selector: "span" })).toBeNull();
    });

    it("reads a signed submission back with its answers and signature", async () => {
      const view: SubmissionView = {
        submission: aSubmission(),
        form: aForm({ fields: [field(), field({ key: "sig", label: "Sign here", type: "signature" })] }),
        integrityOk: true,
        signatureImage: "data:image/png;base64,AAAA",
      };
      getSubmission.mockResolvedValue(view);
      render(<FormsPage />);
      await screen.findByText("Health intake");
      await openResponses();

      fireEvent.click(await screen.findByRole("button", { name: /read it/i }));

      const dialog = await screen.findByRole("dialog");
      expect(within(dialog).getByText("Any allergies?")).toBeTruthy();
      expect(within(dialog).getByText("None")).toBeTruthy();
      expect(within(dialog).getByText("Ama Kwarteng")).toBeTruthy();
      expect(within(dialog).getByText(/matches its signature/i)).toBeTruthy();
      // The signature field is the act of signing, not a question to list.
      expect(within(dialog).queryByText("Sign here")).toBeNull();
    });

    it("says plainly when a record no longer matches its signature", async () => {
      getSubmission.mockResolvedValue({
        submission: aSubmission(),
        form: aForm(),
        integrityOk: false,
      });
      render(<FormsPage />);
      await screen.findByText("Health intake");
      await openResponses();

      fireEvent.click(await screen.findByRole("button", { name: /read it/i }));

      const dialog = await screen.findByRole("dialog");
      expect(within(dialog).getByRole("alert").textContent).toMatch(/changed since it was signed/i);
    });

    it("shows an unanswered optional question as such, not as blank", async () => {
      getSubmission.mockResolvedValue({
        submission: aSubmission({ answers: {} }),
        form: aForm({ fields: [field({ required: false })] }),
        integrityOk: true,
      });
      render(<FormsPage />);
      await screen.findByText("Health intake");
      await openResponses();

      fireEvent.click(await screen.findByRole("button", { name: /read it/i }));

      const dialog = await screen.findByRole("dialog");
      expect(within(dialog).getByText("Not answered")).toBeTruthy();
    });

    it("offers no way to change a submitted answer", async () => {
      getSubmission.mockResolvedValue({
        submission: aSubmission(),
        form: aForm(),
        integrityOk: true,
      });
      render(<FormsPage />);
      await screen.findByText("Health intake");
      await openResponses();

      fireEvent.click(await screen.findByRole("button", { name: /read it/i }));

      const dialog = await screen.findByRole("dialog");
      // A consent record that can be edited after signing is not one.
      expect(within(dialog).queryAllByRole("textbox")).toHaveLength(0);
      expect(within(dialog).queryByRole("button", { name: /save|edit/i })).toBeNull();
    });
  });
});
