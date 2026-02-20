import { StaticPage } from "../components/StaticPage"

export default function Terms() {
  return (
    <StaticPage title="Terms & Conditions">
      <p>
        <strong>Effective Date:</strong> January 1, 2026
      </p>

      <h2>1. Acceptance of Terms</h2>
      <p>
        By accessing or using SecretDrop ("Service"), you agree to be bound by these Terms &
        Conditions. If you do not agree, do not use the Service.
      </p>

      <h2>2. Description of Service</h2>
      <p>
        SecretDrop is a secure, one-time secret sharing platform. Secrets are encrypted using
        AES-256-GCM, stored temporarily, and permanently deleted after being viewed or upon
        expiration.
      </p>

      <h2>3. User Accounts</h2>
      <p>
        You may sign in using third-party authentication providers (Google, GitHub). You are
        responsible for maintaining the security of your account and all activities that occur under
        it.
      </p>

      <h2>4. Acceptable Use</h2>
      <p>You agree not to use the Service to:</p>
      <ul>
        <li>Transmit unlawful, harmful, or offensive content</li>
        <li>Violate any applicable laws or regulations</li>
        <li>Attempt to circumvent security measures</li>
        <li>Abuse the Service through automated or excessive requests</li>
      </ul>

      <h2>5. Data Retention</h2>
      <p>
        Secrets are deleted after a single viewing or upon expiration, whichever comes first. We do
        not retain decrypted content. Account information is retained as long as your account is
        active.
      </p>

      <h2>6. Limitation of Liability</h2>
      <p>
        The Service is provided "as is" without warranties of any kind. Bilustek, LLC shall not be
        liable for any indirect, incidental, or consequential damages arising from the use of the
        Service.
      </p>

      <h2>7. Changes to Terms</h2>
      <p>
        We reserve the right to modify these terms at any time. Continued use of the Service after
        changes constitutes acceptance of the new terms.
      </p>

      <h2>8. Contact</h2>
      <p>
        Questions about these terms may be directed to{" "}
        <a href="mailto:support@bilusteknoloji.com">support@bilusteknoloji.com</a>.
      </p>
    </StaticPage>
  )
}
