# Spec: Frontend Authentication, RBAC & Multi-Library Tenancy

| | |
|---|---|
| **Status** | `APPROVED` (Maintainer Gate 1 sign-off 2026-09-02) |
| **Phase** | `12-auth` |
| **Author** | Claude (Sonnet 4.6), approved by Luann Moreira |
| **Created** | 2026-09-02 |
| **Last updated** | 2026-09-02 |
| **Supersedes** | — |
| **Reviewed in** | Gate 1 Review Batch |

## Context

The Alexandryn frontend (Web and Electron desktop) must provide a seamless, secure, and accessible user experience for authentication, initial admin bootstrap, MFA verification, multi-library switching, and role-gated UI controls.

## User Flows & Screens

### 1. Setup Wizard (First Run)
- Triggered automatically when `GET /api/v1/auth/setup/status` returns `{ isSetup: false }`.
- Route: `/setup`.
- Clean, focused view prompting the administrator to create the master `admin` account (Username, Email, Password, Confirm Password).
- On success, stores tokens, initializes active library to Default Library, and navigates to the library dashboard.

### 2. Login & MFA Verification
- Route: `/login`.
- Standard login form with Email/Username and Password fields.
- Supports "Remember Me" session persistence.
- If response indicates `mfaRequired: true`, transitions into the **TOTP MFA Prompt**:
  - 6-digit TOTP input (with auto-focus, paste support, and digit grouping).
  - Option to switch to "Use backup recovery code".
  - Submits to `/api/v1/auth/mfa/totp/verify` with `mfaTicket`.

### 3. Library Switcher & Management
- Header / Navigation bar displays the current active library with a dropdown menu.
- Dropdown lists all user libraries with active badge.
- Includes "Create Library" action for administrators.
- Allows switching active library context, updating the `X-Library-Id` header across all TanStack Query requests.

### 4. Role-Aware UI Gating
- Navigation and action controls adapt to the authenticated user's role and library ingestion settings:
  - **Sources Management** (`/sources`): Only visible and accessible to `admin`. Hidden for `reader`.
  - **Book Import / Upload**:
    - If user is `admin`: Always available.
    - If user is `reader`: Only available if active library has `allowReaderUploads === true`. Otherwise hidden or disabled with clear explanatory tooltip.
  - **Library Settings & Members**: Only visible to library admins.

### 5. TOTP MFA Setup in User Settings
- Settings screen contains a dedicated "Security & 2FA" section.
- Admin/user can click "Enable Two-Factor Authentication":
  - Displays QR code (rendered via SVG matrix), textual base32 secret, and a confirmation code input.
  - Generates and presents 8 single-use recovery codes with a "Copy Codes" button.
  - Requires entering a valid 6-digit code to complete enrollment.

## Accessibility (WCAG 2.1 AA)

- All form inputs have explicit `<label>` bindings, `aria-required`, and descriptive `aria-invalid` / error announcements (`aria-live="polite"`).
- MFA 6-digit input is fully operable via keyboard, supporting Backspace, Arrow keys, and full string pasting.
- Visible focus rings (`focus-visible:ring-2`) on all interactive controls.
- Color contrast exceeds 4.5:1 on all text elements and form controls.

## Acceptance Criteria

- [ ] Setup screen appears on fresh instance and registers admin.
- [ ] Login screen authenticates user and seamlessly handles TOTP verification.
- [ ] Library switcher updates active library context and filters all views.
- [ ] Reader role properly restricts access to source configuration and upload buttons when disabled.
- [ ] Security settings allow enabling/disabling TOTP MFA with recovery codes.
