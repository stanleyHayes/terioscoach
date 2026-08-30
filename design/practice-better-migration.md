# Practice Better source-flow mapping

The supplied Practice Better flowsheet describes the operating workflow that inspired the Terios application. Terios now implements these responsibilities directly, so the external setup guide is retained here as a requirements map rather than shipped as an operator instruction.

| Supplied workflow | Terios implementation |
|---|---|
| Practitioner profile, timezone, and company details | Admin settings and production configuration |
| Logo, colors, website, email, and PDF branding | `design/brand.md`, web/admin design tokens, and email templates |
| Calendar availability and time blocking | Admin availability plus live booking slots |
| Client service selection and recurring offerings | Admin service catalog and public booking flow |
| Intake forms, waivers, and questionnaires | Admin forms, assignments, and client portal completion |
| Nurse or holistic coaching agreement | Counsel-reviewed templates, then configured consent/form flow |
| Appointment reminders and confirmations | Transactional email templates and scheduled reminders |
| Client records and session notes | Secure practitioner dashboard and client portal |
| Payment setup | Terios payment configuration and booking checkout |

The original guide recommends Practice Better's Plus plan and later Stripe integration. Those vendor-specific steps are not production instructions for Terios. Current deployment, payment, privacy, and go-live procedures are maintained in the repository runbooks.

Before production use, legal counsel must customize the agreement templates, confirm governing law, validate HIPAA/privacy language for the actual operating entity and jurisdictions, and approve cancellation, refund, late-payment, minor-consent, and liability terms.
