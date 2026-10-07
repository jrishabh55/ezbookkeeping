# SMS auto-capture: the iOS shortcut

iOS gives no app access to SMS. Bank alerts reach ezBookkeeping through a Shortcuts automation that
posts each matching message to `POST /api/v1/alerts/ingest.json` with the user's ingest-only token
(created in Settings → SMS Auto-capture → Set Up). The token can do nothing except record alerts for
its owner, and creating a new one revokes the old one.

## Build the shared shortcut (once, by the server owner)

Create a shortcut named **Record bank SMS**:

1. **Receive** Messages input (from automation / Share Sheet). If there is no input: Stop.
2. **Get Details of Messages** → *Sender* (saved as `Sender`).
3. **Get Details of Messages** → *Content* (saved as `Body`).
4. **Text** → the setup code. Leave it empty in the shared copy; it becomes the import question.
5. **Get Contents of URL**
   - URL: `https://<your-server>/api/v1/alerts/ingest.json`
   - Method: POST
   - Headers: `Authorization` = `Bearer ` + the Text from step 4, `Content-Type` = `application/json`
   - Request Body: JSON, `sender` = `Sender`, `text` = `Body`
     (`receivedAt` is optional; the server uses the time it receives the request)
6. **Get Dictionary Value** `success` from the result.
   **If** it is not `true` → **Show Notification** "Couldn't record this SMS, it will be picked up from the statement".

Share it: Share → **Copy iCloud Link**, with **Set Up Import Questions** asking
"Paste your setup code" for the Text action in step 4. Open the link on another device and check that
the copy contains no token.

Set the link on the server so the settings page can offer it:

```ini
[alerts]
shortcut_url = https://www.icloud.com/shortcuts/...
```

or the environment variable `EBK_ALERTS_SHORTCUT_URL`.

## Per user, on the iPhone

1. ezBookkeeping → Settings → **SMS Auto-capture** → **Set Up**, confirm the password, **Copy** the code.
2. **Install Shortcut**, and paste the code when asked.
3. Shortcuts → Automation → **New** → **Message** → *Message Contains* `Rs` → **Run Immediately**
   (Ask Before Running off) → Run Shortcut **Record bank SMS**.
4. Repeat step 3 with *Message Contains* `INR`; some banks write only INR. A message that contains
   both is posted twice and recorded (or counted) once.
5. Back in the app, **Send Test**, then wait for the next bank SMS (or forward one to yourself).

Each request answers `added`, `duplicate`, `ignored` (OTP, promotions, reminders; only counted, the
message text is not kept) or `unparsed` (looked financial, could not be read safely). The last 10
unparsed messages are listed on Settings → SMS Auto-capture under "Couldn't read — add these
manually". Anything missed is filled in by the next statement import.
