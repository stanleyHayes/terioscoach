export interface HelpTopic {
  path: string;
  title: string;
  goal: string;
  steps: string[];
  tip?: string;
}

export const PORTAL_HELP_TOPICS: HelpTopic[] = [
  { path: "/portal", title: "Overview", goal: "See your next care actions and move quickly to the right record.", steps: ["Review your next appointment and outstanding actions.", "Open any form, payment, or session that needs attention.", "Use the navigation to return to your complete care history."] },
  { path: "/portal/book", title: "Book a session", goal: "Choose the right service and reserve an available time.", steps: ["Choose a published service.", "Pick an available date and time in your displayed timezone.", "Review the details, confirm the booking, and complete payment when requested."], tip: "If no time appears, try another date. The practice must publish both a service and future availability before a slot can be booked." },
  { path: "/portal/sessions", title: "Consultations", goal: "Manage upcoming appointments and review completed sessions.", steps: ["Open an upcoming session to review its time and status.", "Reschedule or cancel within the displayed policy window.", "Use Join session when the room opens ten minutes before the appointment."] },
  { path: "/portal/sessions/:id/room", title: "Video consultation", goal: "Join your private call and control what you share.", steps: ["Check your camera and microphone, then join the waiting room.", "Wait for the practitioner to admit you.", "Approve or decline recording requests explicitly, and choose Leave only when you are finished."] },
  { path: "/portal/forms", title: "Forms", goal: "Complete requested care information and sign it accurately.", steps: ["Open a form marked as waiting for you.", "Complete every required field and review your answers.", "Add your signature and submit; signed submissions cannot be edited afterward."] },
  { path: "/portal/documents", title: "Documents", goal: "Access files that belong to your care record.", steps: ["Review the document name and upload date.", "Open or download the file you need.", "Contact the practice if a document appears missing or incorrect."] },
  { path: "/portal/payments", title: "Payments", goal: "Complete outstanding payment and keep track of past transactions.", steps: ["Review any item marked pending first.", "Choose Pay now to continue through the secure payment page.", "Return here to confirm successful or refunded transactions."] },
  { path: "/portal/reviews", title: "Reviews", goal: "Share useful feedback after an eligible session.", steps: ["Choose a completed session that has not been reviewed.", "Select a rating and add an optional comment.", "Submit once; the practice reviews feedback before anything is published publicly."] },
  { path: "/portal/settings", title: "Profile & preferences", goal: "Keep your account details, password, and experience preferences current.", steps: ["Update your name and save the profile.", "Change your password using the current password.", "Set preferences or restart the onboarding tutorial when you want guidance again."] },
  { path: "/portal/guide", title: "User guide", goal: "Learn how to complete every main client-portal task.", steps: ["Find the task you want to complete.", "Follow its short ordered guide.", "Return to the live page and use the Help button whenever you need context."] },
];

export function portalHelpForPath(pathname: string): HelpTopic {
  if (/^\/portal\/sessions\/[^/]+\/room(?:\/|$)/.test(pathname)) {
    return PORTAL_HELP_TOPICS.find((topic) => topic.path === "/portal/sessions/:id/room")!;
  }
  return [...PORTAL_HELP_TOPICS]
    .sort((a, b) => b.path.length - a.path.length)
    .find((topic) => pathname === topic.path || pathname.startsWith(`${topic.path}/`))
    ?? PORTAL_HELP_TOPICS[0]!;
}
