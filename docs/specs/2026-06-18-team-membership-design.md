# Team Membership API — Functional Spec

> **Superseded (2026-06-25).** The shipped Modjo Public API v2 resolved team
> membership on the **user** resource, not the team: `POST /users/{id}/teams`
> (body `{teamId}`) and `DELETE /users/{id}/teams/{teamId}` — neither Option A
> (`PUT /teams/{id}/members/{userId}`) nor Option B below. The CLI implements
> the shipped shape as `modjo users add-team` / `modjo users remove-team`. This
> doc is kept for historical context only.

Status: Draft · 2026-06-18

## Problem

The Modjo Public API v2 can read team membership but cannot change it.

- `GET /teams/{id}/members` lists a team's members.
- `POST /teams`, `PATCH /teams/{id}`, `DELETE /teams/{id}` manage teams.
- `POST /users`, `PATCH /users/{id}`, `DELETE /users/{id}` manage users.

There is **no endpoint to add a user to a team or remove one**. Membership can
only be set through the Modjo UI. Customers automating user provisioning have to
do it by hand.

_(Backstory / motivation: to be filled in.)_

## Solution

Add membership write operations under the existing `/teams/{id}/members`
sub-resource. Two options are proposed; they differ only on the **add**
operation. The **remove** operation is identical in both.

### Option A — Idempotent PUT _(preferred)_

Membership is a resource addressed by `(team, user)`. PUT asserts the fact "user
is in team"; it is safe to retry and has no "already a member" error case.

```
PUT    /teams/{teamId}/members/{userId}   → 200 TeamMember   (idempotent)
DELETE /teams/{teamId}/members/{userId}   → 204
```

- No request body required.
- Re-adding an existing member returns 200, not an error.
- Removing a non-member is a no-op (204) or 404 — see Open Questions.
- Follows the widely-used GitHub team-membership shape.

### Option B — Sub-collection POST

Mirrors Modjo's existing `/calls/{id}/tags` write pattern for internal
consistency.

```
POST   /teams/{teamId}/members            → 201 TeamMember
       body: { "userId": <number> }
DELETE /teams/{teamId}/members/{userId}   → 204
```

- Add takes the user id in the request body, like
  `POST /calls/{id}/tags` takes `{ "tagId" }`.
- Re-adding an existing member must return an error (e.g. 409 Conflict).

### Shared details

- Both reuse the existing `TeamMember` schema (same shape as `User`).
- Both keep the existing `GET /teams/{id}/members` list endpoint unchanged.
- Errors follow the current convention: `{code, message}`, with 404 when the
  team or user does not exist.

### Trade-off summary

| | A — PUT (preferred) | B — POST |
|---|---|---|
| Consistency with broader REST world (GitHub, etc.) | high | medium |
| Consistency with existing Modjo writes (`/calls/{id}/tags`) | medium | high |
| Idempotent / retry-safe add | yes | no |
| "Already a member" handling | not a case | must return 409 |
| User id location | URL path | request body |

## Open Questions

1. **Cardinality** — can a user belong to multiple teams at once, or only one?
   If only one, adding to a new team would move the user, and a
   `teamId` field on the user could be an alternative to a membership
   sub-collection.
2. **Remove a non-member** — should `DELETE` on a non-member return 204
   (idempotent) or 404 (not found)?
3. **Bulk** — do we need to add/remove several users in one call (Zendesk /
   Front offer this), or is single-user enough for now?
4. **Membership metadata** — does a membership ever carry its own fields (role
   within the team, added-by, added-on), or is it purely a link? This affects
   whether the PUT body is empty or not.
5. **CLI surface** — what should the `modjo teams` commands look like
   (e.g. `modjo teams members add/remove`)? Out of scope for the API spec but
   needed before implementation.

## References

- Current spec: `docs/modjo-openapi.json`
- Existing relationship-write precedent: `POST /calls/{id}/tags` /
  `DELETE /calls/{id}/tags/{tagId}`
- GitHub team membership: `PUT /orgs/{org}/teams/{slug}/memberships/{username}`
