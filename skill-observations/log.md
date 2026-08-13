# Skill Observation Log

Observations captured during task-oriented work.

**Status key:** OPEN = not yet actioned | ACTIONED (YYYY-MM-DD) = skill updated/created | DECLINED (YYYY-MM-DD) = user decided not to pursue — resolved statuses always carry their resolution date

---

2026-08-12 checkpoint — no observations.

2026-08-12 deliverable flush — no observations.

## 2026-08-12

### Observation 1: Complete redesigns need route-level proof

**Status:** OPEN
**Date:** 2026-08-12
**Session context:** A product-wide redesign was rejected because shared primitives changed while many route interiors remained structurally familiar.
**Skill:** redesign-existing-projects
**Type:** open-source
**Phase/Area:** Scan, diagnose, verification

**Issue:** The workflow can declare a redesign complete after global token and shell improvements without proving that each reachable route, workflow, and state received meaningful design judgment.

**Suggested improvement:** Require a route-and-state matrix before implementation, at least one structural change per materially distinct route family, and rendered desktop/mobile evidence before completion.

**Principle:** Product-wide redesign scope must be proven per surface, not inferred from shared-component coverage.

### Observation 2: Rendered palette verification belongs in the redesign loop

**Status:** OPEN
**Date:** 2026-08-12
**Session context:** New branded utility classes were present in markup and builds passed, but the app had not exposed the corresponding design tokens to its CSS framework.
**Skill:** redesign-existing-projects
**Type:** open-source
**Phase/Area:** Fix and verification

**Issue:** Static class review and successful compilation did not reveal that several intended colors had no effect, leaving the rendered result visually close to the old design.

**Suggested improvement:** After changing palette usage, inspect computed styles for signature colors on a rendered page and verify each app maps the full token set before expanding the design.

**Principle:** A visual token is not implemented until its computed style is verified in the browser.

### Observation 3: Route motion belongs inside persistent application chrome

**Status:** OPEN
**Date:** 2026-08-12
**Session context:** Adding page transitions to a multi-layout application with persistent public, portal, and dashboard shells.
**Skill:** emil-design-eng
**Type:** open-source
**Phase/Area:** Route transitions

**Issue:** A root-level transition template also remounts and animates persistent navigation, making route changes feel like full reloads instead of spatially continuous navigation.

**Suggested improvement:** In the route-transition guidance, require locating templates at the narrowest route segment beneath persistent chrome and explicitly verify that navigation and sidebars do not replay on each transition.

**Principle:** Page transitions should animate changing content while stable application chrome remains visually anchored.

### Observation 4: Verify scroll-state navigation in a real browser

**Status:** OPEN
**Date:** 2026-08-12
**Session context:** Replicating a settled-to-floating header behavior across applications.
**Skill:** redesign-existing-projects
**Type:** open-source
**Phase/Area:** Interaction verification

**Issue:** The scroll-state contract passed component tests while an animation-frame scheduled update remained stale in a real background-restored browser tab.

**Suggested improvement:** Add an explicit real-browser check for top, past-threshold, and returned-to-top states whenever navigation changes shape on scroll; prefer a direct passive listener when the state calculation is only one boolean.

**Principle:** A scroll-responsive shell is complete only when its state transitions are verified against actual browser scroll position.

### Observation 5: Card redesigns need semantic families

**Status:** OPEN
**Date:** 2026-08-12
**Session context:** A shared card primitive had been polished, but service choices and portal records still felt generic because every content type retained the same large bordered-box composition.
**Skill:** redesign-existing-projects
**Type:** open-source
**Phase/Area:** Component systems

**Issue:** Applying one elevated card treatment everywhere can improve finish without improving meaning or product identity.

**Suggested improvement:** During a redesign, inventory cards by job—choice, record, summary, quote, feature, empty/error—and establish a distinct hierarchy and silhouette for each repeated family before styling individual routes.

**Principle:** A coherent card system repeats a visual language, not one universal rectangle.

2026-08-12 legal-page redesign checkpoint — no new skill observations.

2026-08-12 password-recovery checkpoint — no new skill observations.

2026-08-12 production-hardening checkpoint — no new skill observations.
