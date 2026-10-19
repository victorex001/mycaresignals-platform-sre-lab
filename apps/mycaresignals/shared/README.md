# Shared code

This area is reserved for code that is genuinely reusable across the staff and
feedback applications and is not security-sensitive.

Staff domain logic stays in `../staff/`. Public-feedback behavior and its
security controls stay in `../feedback/`. Database-specific logic must not be
moved here merely to remove duplication; sharing must preserve the separate
services, data stores, permissions, and QR-to-TE intake boundary.

No existing application code is shared during Stage 1.
