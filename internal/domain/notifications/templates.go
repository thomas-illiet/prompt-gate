package notifications

import (
	"fmt"
	"html"
	"strings"
)

type emailTemplate struct {
	kicker       string
	title        string
	intro        string
	supporting   string
	details      string
	image        string
	imageAlt     string
	imageCaption string
	action       string
	actionPath   string
	accent       string
}

func templateFor(kind Kind, recipientName, subject string) emailTemplate {
	templates := map[Kind]emailTemplate{
		KindWelcome: {
			kicker: "First time in", title: "Welcome. Everything is ready.",
			intro:      "Your access is ready, your routes are in place, and your first prompt can go.",
			supporting: "Take a minute to explore your dashboard and check the models available to you. Prompt Gate will keep usage, quotas, and access details together as you get started.",
			details:    "Start here", image: "welcome.jpg",
			imageAlt: "A cheerful person relaxing in a soft green lounge.", imageCaption: "Entry approved. Smiling strongly encouraged.",
			action: "Send my first prompt", actionPath: "/", accent: "#0b8b5a",
		},
		KindQuotaExhausted: {
			kicker: "Quota paused", title: "Your quota is taking a little nap.",
			intro:      "Nothing is broken. Prompt Gate is simply waiting for capacity to become available again.",
			supporting: "Your account, history, and settings remain unchanged while requests are paused. You can review the affected window below and decide whether to use a reset or wait for capacity to return.",
			details:    "What happens now", image: "quota-exhausted.jpg",
			imageAlt: "A tired person holding an empty battery indicator.", imageCaption: "It gave everything it had.",
			action: "Review my quota", actionPath: "/profile", accent: "#d45b4b",
		},
		KindQuotaReset: {
			kicker: "Reset added", title: "A reset just landed.",
			intro:      "You have one more way to get back to work when a quota window runs out.",
			supporting: "The credit stays available in your account until you choose to use it. Nothing changes immediately, so you can keep it for the moment you need it most.",
			details:    "Reset details", image: "quota-reset-granted.jpg",
			imageAlt: "A playful bonus token next to a recharged battery.", imageCaption: "The gauge goes up. The drama goes down.",
			action: "View my resets", actionPath: "/profile", accent: "#c28320",
		},
		KindGroupChanged: {
			kicker: "Access updated", title: "New team, new doors.",
			intro:      "Your group membership changed, so the models and limits available to you may have changed too.",
			supporting: "No action is required, but it is worth checking your current access before your next project. Your profile always shows the groups and capabilities that apply now.",
			details:    "Access summary", image: "group-changed.jpg",
			imageAlt: "Soft puzzle pieces fitting together.", imageCaption: "The right piece just found its place.",
			action: "Review my access", actionPath: "/profile", accent: "#0b7f67",
		},
		KindTokenCreated: {
			kicker: "New key", title: "A brand-new key. Keep it safe.",
			intro:      "Your new virtual key is active. Store it securely and rotate it before it expires.",
			supporting: "Prompt Gate will not display the secret again after creation. Keep it in a trusted secret manager, share it only with the integration that needs it, and remove older keys when they are no longer used.",
			details:    "Key details", image: "token-created.jpg",
			imageAlt: "A bright golden key protected by a soft character.", imageCaption: "Big key. Close protection.",
			action: "Manage my keys", actionPath: "/tokens", accent: "#168761",
		},
		KindTokenExpired: {
			kicker: "Key expired", title: "This key has finished its shift.",
			intro:      "Integrations using it now need a replacement key before they can authenticate again.",
			supporting: "The rest of your account and active keys are unaffected. Create a replacement only for integrations that still need access, then update their secret before restarting them.",
			details:    "Replace the key", image: "token-expired.jpg",
			imageAlt: "A retired golden key resting in a small armchair.", imageCaption: "The key handed in its badge and left for the weekend.",
			action: "Create a new key", actionPath: "/tokens", accent: "#c28320",
		},
		KindTokenRevoked: {
			kicker: "Access blocked", title: "This key is not getting through.",
			intro:      "The revoked key can no longer access Prompt Gate or authenticate new requests.",
			supporting: "Any integration still using it will begin to fail authentication. If the revocation was intentional, no further action is needed; otherwise, review your keys and contact an administrator.",
			details:    "Security check", image: "token-revoked.jpg",
			imageAlt: "A golden key stopped at a security gate.", imageCaption: "The gate said no. Twice.",
			action: "Review my keys", actionPath: "/tokens", accent: "#d45b4b",
		},
		KindMonthlyReport: {
			kicker: "Monthly report", title: "Your month, by the numbers — but fun.",
			intro:      "A quick, readable view of your Prompt Gate activity — no sad spreadsheet required.",
			supporting: "Use this snapshot to spot changes in request volume, model choices, and response time. The dashboard keeps the full picture if you want to explore a trend in more detail.",
			details:    "At a glance", image: "monthly-usage-report.jpg",
			imageAlt: "A playful tactile dashboard with charts and counters.", imageCaption: "When the bars move in the right direction.",
			action: "Explore my dashboard", actionPath: "/dashboard", accent: "#0e7767",
		},
		KindManual: {
			kicker: "Message from Prompt Gate", title: subject,
			intro:      "A direct update from the team that keeps your routes running.",
			supporting: "You will find the full context below, including anything that changes and any action expected from you. If no action is mentioned, this message is simply keeping you informed.",
			details:    "Message", image: "manual.jpg",
			imageAlt: "Small friendly robots reopening a route after maintenance.", imageCaption: "Good routes are worth sharing.",
			action: "Open Prompt Gate", actionPath: "/", accent: "#146f55",
		},
	}
	template, ok := templates[kind]
	if !ok {
		template = templates[KindManual]
	}
	if kind == KindWelcome && strings.TrimSpace(recipientName) != "" {
		template.title = "Welcome, " + strings.TrimSpace(recipientName) + ". Everything is ready."
	}
	if strings.TrimSpace(template.title) == "" {
		template.title = "A message from Prompt Gate"
	}
	return template
}

func renderEmailTemplate(template emailTemplate, subject, contentHTML, frontendURL string) string {
	imageURL := publicEmailURL(frontendURL, "/email/"+template.image)
	actionURL := publicEmailURL(frontendURL, template.actionPath)
	imageBlock := ""
	if imageURL != "" {
		imageBlock = fmt.Sprintf(`<tr><td style="padding:0"><img class="pg-hero" src="%s" width="620" height="310" alt="%s" style="display:block;width:100%%;max-width:620px;height:auto;aspect-ratio:2/1;object-fit:cover;object-position:center;border:0"><p style="margin:0;padding:11px 28px 12px;background:#18382f;color:#ffffff;font-size:13px;line-height:20px">%s</p></td></tr>`, html.EscapeString(imageURL), html.EscapeString(template.imageAlt), html.EscapeString(template.imageCaption))
	}
	actionBlock := ""
	if actionURL != "" {
		actionBlock = fmt.Sprintf(`<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin-top:26px"><tr><td style="border-radius:12px;background:#173f34"><a href="%s" style="display:inline-block;padding:14px 20px;color:#ffffff;font-size:14px;font-weight:700;line-height:18px;text-decoration:none">%s</a></td></tr></table>`, html.EscapeString(actionURL), html.EscapeString(template.action))
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light"><title>%s</title>
<style>@media only screen and (max-width:640px){.pg-shell{width:100%%!important}.pg-pad{padding-left:22px!important;padding-right:22px!important}.pg-title{font-size:29px!important;line-height:34px!important}}.pg-content h1,.pg-content h2,.pg-content h3{margin:0 0 10px;color:#18382f;font-size:17px;line-height:24px}.pg-content p{margin:0;padding:0 0 14px}.pg-content p+p{padding-top:14px;border-top:1px solid #dfe6df}.pg-content p:last-child{padding-bottom:0}.pg-content strong{color:#18382f}.pg-content ul{margin:0;padding:0;list-style:none}.pg-content li{margin:0;padding:12px 0;border-top:1px solid #dfe6df}.pg-content li:first-child{padding-top:0;border-top:0}.pg-content li:last-child{padding-bottom:0}.pg-content ol{margin:0;padding-left:22px}.pg-content a{color:#0b7656}</style></head>
<body style="margin:0;padding:0;background:#eef2eb;color:#263c35;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif">
<div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent">%s</div>
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="width:100%%;background:#eef2eb"><tr><td align="center" style="padding:32px 14px">
<table class="pg-shell" role="presentation" width="620" cellspacing="0" cellpadding="0" border="0" style="width:620px;max-width:620px;background:#fffdf7;border:1px solid #d7e0d7;border-radius:22px;overflow:hidden">
<tr><td style="padding:22px 28px;background:#18382f;color:#ffffff"><strong style="font-size:20px;line-height:24px">Prompt<span style="color:#66d3a3">Gate</span></strong><span style="float:right;padding-top:4px;color:#c8ddd4;font-size:11px;letter-spacing:1.2px;text-transform:uppercase">Good routes only</span></td></tr>
%s
<tr><td class="pg-pad" style="padding:34px 38px 38px"><p style="margin:0 0 9px;color:%s;font-size:11px;font-weight:700;letter-spacing:1.4px;text-transform:uppercase">%s</p><h1 class="pg-title" style="margin:0 0 13px;color:#18382f;font-size:34px;line-height:39px;font-weight:700;letter-spacing:-0.7px">%s</h1><p style="margin:0;color:#53665f;font-size:16px;line-height:25px">%s</p><p style="margin:13px 0 0;color:#6a7b75;font-size:14px;line-height:22px">%s</p>
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="width:100%%;margin-top:30px;border-top:1px solid #d7e0d7"><tr><td style="padding:22px 0 0"><p style="margin:0 0 14px;color:%s;font-size:11px;font-weight:700;letter-spacing:1.2px;text-transform:uppercase">%s</p><div class="pg-content" style="color:#3f554d;font-size:15px;line-height:23px">%s</div></td></tr></table>%s</td></tr>
<tr><td style="padding:17px 28px;border-top:1px solid #d7e0d7;color:#6b7d76;font-size:11px;line-height:17px">Prompt Gate · Secure model access<br><a href="%s" style="color:#0b7656;text-decoration:none">Manage your account</a></td></tr>
</table></td></tr></table></body></html>`,
		html.EscapeString(subject), html.EscapeString(template.intro), imageBlock, html.EscapeString(template.accent), html.EscapeString(template.kicker), html.EscapeString(template.title), html.EscapeString(template.intro), html.EscapeString(template.supporting), html.EscapeString(template.accent), html.EscapeString(template.details), contentHTML, actionBlock, html.EscapeString(publicEmailURL(frontendURL, "/profile")))
}

func publicEmailURL(frontendURL, path string) string {
	frontendURL = strings.TrimRight(strings.TrimSpace(frontendURL), "/")
	if frontendURL == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return frontendURL + path
}
