# Invoice PDF download and Codeation layout

The invoice download must save a generated PDF directly. Printing remains a separate action named **Print**. The user supplied `Invoice_14_2020-06-04.pdf` as the visual reference and confirmed that logo visibility applies globally through Invoice Profile.

Use the existing Go PDF renderer rather than browser printing or a second renderer. Download its bytes through an authenticated API request, validate the PDF response, and save a named file. Use the same renderer for public downloads and email attachments.

Match the reference's blue page border, Codeation wordmark at top left, business details at top right, serif typography, decorative Invoice frame, ruled services table and totals. Keep currency, GST, decimal quantities, payment totals and the authenticity footer. Do not copy the reference's customer details, terms or Wave branding into invoices.

Keep PNG/JPEG upload in Invoice Profile. Add **Show logo on invoices**, on by default. Store `hideLogo=false` by default so existing profiles retain their appearance. Hiding a logo preserves its uploaded image, suppresses it in the preview/PDF/email attachment, and omits its image data from public responses. Desktop and mobile expose the same control. Load the supplied Codeation PNG into the user's live profile after deployment without changing other profile fields.

Verify profile save/reload, hiding and re-enabling without losing the image, direct PDF delivery, rejection of non-PDF responses, public rendering, and multi-page output. Render samples with Poppler for visual review. No quantity migration is needed for this change.
