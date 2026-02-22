import { StaticPage } from "../components/StaticPage"

export default function Privacy() {
  return (
    <StaticPage title="Privacy Policy">
      <p>
        <strong>Effective Date:</strong> January 1, 2026
      </p>

      <h2>1. Information We Collect</h2>
      <p>When you use SecretDrop, we may collect:</p>
      <ul>
        <li>
          <strong>Account information:</strong> Name, email address, and avatar from your
          authentication provider (Google or GitHub)
        </li>
        <li>
          <strong>Usage data:</strong> Number of secrets created and basic access logs
        </li>
        <li>
          <strong>Payment information:</strong> Processed securely through Stripe; we do not store
          card details
        </li>
      </ul>

      <h2>2. Information We Do Not Collect</h2>
      <p>
        We do not collect, store, or have access to the decrypted content of your secrets.
        Encryption keys are never transmitted to our servers.
      </p>

      <h2>3. How We Use Your Information</h2>
      <ul>
        <li>To provide and maintain the Service</li>
        <li>To manage your account and subscription</li>
        <li>To send transactional emails (secret notifications)</li>
        <li>To respond to support inquiries</li>
      </ul>

      <h2>4. Data Sharing</h2>
      <p>
        We do not sell your personal information. We may share data with third-party services
        necessary for operating the platform (e.g., Stripe for payments, Resend for email delivery).
      </p>

      <h2>5. Data Security</h2>
      <p>
        We use industry-standard security measures including AES-256-GCM encryption, secure HTTPS
        connections, and regular security reviews to protect your data.
      </p>

      <h2>6. Data Retention</h2>
      <p>
        Secrets are permanently deleted after viewing or expiration. Account data is retained as long
        as your account remains active.
      </p>

      <h2>7. Account Deletion</h2>
      <p>
        You may delete your account at any time from your account menu. Upon deletion, all account
        data and subscription records are immediately and permanently removed. Previously shared
        secrets are not linked to your account and will continue to expire on their original
        schedule.
      </p>

      <h2>8. Your Rights</h2>
      <p>You have the right to:</p>
      <ul>
        <li>Access the personal data we hold about you</li>
        <li>Request correction or deletion of your data</li>
        <li>Delete your account directly from the Service</li>
        <li>Withdraw consent for data processing</li>
      </ul>

      <h2>9. Changes to This Policy</h2>
      <p>
        We may update this Privacy Policy from time to time. We will notify you of significant
        changes through the Service.
      </p>

      <h2>10. Contact</h2>
      <p>
        For privacy-related inquiries, contact us at{" "}
        <a href="mailto:support@bilustek.com">support@bilustek.com</a>.
      </p>
    </StaticPage>
  )
}
