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

      <h2>4. Plans & Pricing</h2>
      <p>The Service offers two tiers:</p>
      <ul>
        <li>
          <strong>Free:</strong> 5 secrets for the lifetime of your account. Once used, no additional
          secrets can be created unless you upgrade.
        </li>
        <li>
          <strong>Pro ($2.99/month):</strong> Up to 100 secrets per billing period. Usage resets at
          the start of each billing cycle.
        </li>
      </ul>

      <h2>5. Subscription & Cancellation</h2>
      <p>
        Pro subscriptions are billed monthly through Stripe. You may cancel your subscription at any
        time via the billing portal accessible from your account menu. Upon cancellation, you retain
        Pro access and your remaining secrets until the end of your current billing period. Once the
        period ends, your account reverts to the Free tier.
      </p>

      <h2>6. Refund Policy</h2>
      <p>
        All payments are non-refundable. When you cancel, you retain access to Pro features until the
        end of your current billing period. No prorated refunds are issued for unused portions of a
        billing cycle.
      </p>

      <h2>7. Account Deletion</h2>
      <p>
        You may permanently delete your account at any time from your account menu. Deleting your
        account will:
      </p>
      <ul>
        <li>Immediately remove your account data and subscription records</li>
        <li>Cancel any active Pro subscription</li>
        <li>
          Previously shared secrets are not linked to your account and will expire on their original
          schedule
        </li>
      </ul>
      <p>This action is irreversible.</p>

      <h2>8. Acceptable Use</h2>
      <p>You agree not to use the Service to:</p>
      <ul>
        <li>Transmit unlawful, harmful, or offensive content</li>
        <li>Violate any applicable laws or regulations</li>
        <li>Attempt to circumvent security measures</li>
        <li>Abuse the Service through automated or excessive requests</li>
      </ul>

      <h2>9. Data Retention</h2>
      <p>
        Secrets are deleted after a single viewing or upon expiration, whichever comes first. We do
        not retain decrypted content. Account information is retained as long as your account is
        active.
      </p>

      <h2>10. Limitation of Liability</h2>
      <p>
        The Service is provided "as is" without warranties of any kind. Bilustek, LLC shall not be
        liable for any indirect, incidental, or consequential damages arising from the use of the
        Service.
      </p>

      <h2>11. Changes to Terms</h2>
      <p>
        We reserve the right to modify these terms at any time. Continued use of the Service after
        changes constitutes acceptance of the new terms.
      </p>

      <h2>12. Contact</h2>
      <p>
        Questions about these terms may be directed to{" "}
        <a href="mailto:support@bilusteknoloji.com">support@bilusteknoloji.com</a>.
      </p>
    </StaticPage>
  )
}
