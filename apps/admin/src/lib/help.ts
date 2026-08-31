export interface HelpTopic {
  path: string;
  title: string;
  goal: string;
  steps: string[];
  tip?: string;
}

export const ADMIN_HELP_TOPICS: HelpTopic[] = [
  { path: "/", title: "Overview", goal: "See what needs attention today and move directly into the relevant workspace.", steps: ["Review today’s sessions and the summary metrics.", "Open any item that needs action.", "Use the recent activity and alerts to confirm nothing has been missed."] },
  { path: "/calendar", title: "Calendar & calls", goal: "Manage appointments and start scheduled consultations.", steps: ["Move to the date or week you need.", "Open a booking to review its client and status.", "Start the video session, reschedule, complete, cancel, or mark a no-show when appropriate."] },
  { path: "/availability", title: "Availability", goal: "Publish the times clients are allowed to book.", steps: ["Enable each working day and set one or more time windows.", "Add a buffer when you need space between sessions.", "Save the week, then add date-specific time off below."], tip: "A published service still has no bookable times until at least one future availability window exists." },
  { path: "/clients", title: "Clients", goal: "Find a client and review their complete care record.", steps: ["Search by name or email.", "Open the client record.", "Review sessions, notes, forms, documents, payments, and shared feedback before taking action."] },
  { path: "/services", title: "Services", goal: "Control what clients can book, including duration and price.", steps: ["Choose New service and enter the name, description, duration, and price.", "Keep the service active to publish it in the booking menu.", "Reorder services or edit them whenever the offer changes."], tip: "For a service to produce bookable sessions, it must be active and Availability must contain future working hours." },
  { path: "/content/posts", title: "Blog editor", goal: "Create a useful article and publish it only after review.", steps: ["Write the title, excerpt, category, cover image, and article body.", "Use Markdown and Preview to check structure and links.", "Save the draft, then publish only when the content is ready for the website."] },
  { path: "/content", title: "Site content", goal: "Keep public pages, articles, FAQs, and testimonials accurate.", steps: ["Choose the relevant content tab.", "Create or edit the item and save it as a draft or pending item.", "Use the separate Publish or Approve action when it is ready to go live."] },
  { path: "/forms", title: "Forms", goal: "Build forms, send them to clients, and review signed responses.", steps: ["Create a form and add the fields the client must complete.", "Assign it to a client or booking.", "Review the submitted record and its integrity status; signed submissions remain read-only."] },
  { path: "/enquiries", title: "Enquiries", goal: "Respond to new questions and keep the inbox resolved.", steps: ["Open unread enquiries first.", "Use the supplied contact details to respond outside the dashboard.", "Mark the enquiry resolved when follow-up is complete."] },
  { path: "/reviews", title: "Reviews", goal: "Moderate client feedback before anything appears publicly.", steps: ["Read the rating and comment in context.", "Approve appropriate feedback or reject content that should remain private.", "Check the public website after approval if publication matters immediately."] },
  { path: "/payments", title: "Payments", goal: "Understand what was collected, pending, or refunded.", steps: ["Choose the reporting month.", "Review totals before opening individual transactions.", "Use Stripe alongside this page when investigating a discrepancy or refund."] },
  { path: "/reports", title: "Reports", goal: "Turn practice activity into a useful operating snapshot.", steps: ["Choose a meaningful date range.", "Review sessions, bookings, income, and service performance.", "Follow unusual totals back to Calendar, Services, or Payments before making a decision."] },
  { path: "/team", title: "Team & access", goal: "Give staff only the access required for their work.", steps: ["Create a role with the minimum necessary permissions.", "Invite or create the staff account and assign that role.", "Review access whenever responsibilities change and deactivate accounts no longer in use."] },
  { path: "/security", title: "Security & MFA", goal: "Protect your account without locking yourself out.", steps: ["Start MFA enrollment only when your authenticator app is ready.", "Scan the QR code and confirm the six-digit code.", "Store recovery codes securely and review active security settings after password changes."] },
  { path: "/settings", title: "Profile & preferences", goal: "Keep your identity, password, display preferences, and tutorial current.", steps: ["Update your name and profile details.", "Change your password using the current password.", "Save preferences or restart the onboarding tutorial whenever you need a refresher."] },
  { path: "/sessions", title: "Video consultation", goal: "Run a private consultation safely from admission through ending the call.", steps: ["Check your camera and microphone, then join.", "Admit the waiting client when you are ready.", "Request consent before recording and use End for everyone when the consultation is complete."] },
  { path: "/guide", title: "User guide", goal: "Learn the complete workflow for operating the practice.", steps: ["Find the workspace you want to understand.", "Follow its goal and ordered steps.", "Return to the live page and use its Help button for contextual guidance."] },
];

export function adminHelpForPath(pathname: string): HelpTopic {
  return [...ADMIN_HELP_TOPICS]
    .sort((a, b) => b.path.length - a.path.length)
    .find((topic) => topic.path === "/" ? pathname === "/" : pathname === topic.path || pathname.startsWith(`${topic.path}/`))
    ?? ADMIN_HELP_TOPICS[0]!;
}
