import { useCallback, useEffect, useState } from "react"
import { StaticPage } from "../components/StaticPage"

type Operator = "+" | "-" | "\u00d7"

interface CaptchaChallenge {
  a: number
  b: number
  op: Operator
  answer: number
}

function generateCaptcha(): CaptchaChallenge {
  const operators: Operator[] = ["+", "-", "\u00d7"]
  const op = operators[Math.floor(Math.random() * operators.length)]
  const a = Math.floor(Math.random() * 9) + 1
  const b = Math.floor(Math.random() * 9) + 1

  let answer: number
  switch (op) {
    case "+":
      answer = a + b
      break
    case "-":
      answer = a - b
      break
    case "\u00d7":
      answer = a * b
      break
  }

  return { a, b, op, answer }
}

export default function Contact() {
  const [name, setName] = useState("")
  const [email, setEmail] = useState("")
  const [message, setMessage] = useState("")
  const [captchaInput, setCaptchaInput] = useState("")
  const [captcha, setCaptcha] = useState<CaptchaChallenge>(generateCaptcha)
  const [status, setStatus] = useState<"idle" | "sending" | "sent" | "error">("idle")
  const [errorMessage, setErrorMessage] = useState("")

  const resetCaptcha = useCallback(() => {
    setCaptcha(generateCaptcha())
    setCaptchaInput("")
  }, [])

  useEffect(() => {
    resetCaptcha()
  }, [resetCaptcha])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    if (parseInt(captchaInput, 10) !== captcha.answer) {
      setErrorMessage("Incorrect answer. Please try again.")
      resetCaptcha()
      return
    }

    setStatus("sending")
    setErrorMessage("")

    try {
      const res = await fetch("/api/v1/contact", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, email, message }),
      })

      if (!res.ok) {
        const body = await res.json()
        throw new Error(body.error?.message || "Failed to send message")
      }

      setStatus("sent")
      setName("")
      setEmail("")
      setMessage("")
      resetCaptcha()
    } catch (err) {
      setStatus("error")
      setErrorMessage(err instanceof Error ? err.message : "Something went wrong")
      resetCaptcha()
    }
  }

  if (status === "sent") {
    return (
      <StaticPage title="Contact">
        <div className="rounded-lg border border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-950 p-6 text-center">
          <p className="text-green-800 dark:text-green-200 font-medium">
            Thank you! Your message has been sent. We'll get back to you soon.
          </p>
        </div>
      </StaticPage>
    )
  }

  return (
    <StaticPage title="Contact">
      <p className="mb-6">
        Have a question or feedback? Fill out the form below and we'll get back to you.
      </p>

      <form onSubmit={handleSubmit} className="space-y-5 not-prose">
        <div>
          <label htmlFor="name" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Name
          </label>
          <input
            id="name"
            type="text"
            required
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
          />
        </div>

        <div>
          <label htmlFor="email" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Email
          </label>
          <input
            id="email"
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
          />
        </div>

        <div>
          <label htmlFor="message" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Message
          </label>
          <textarea
            id="message"
            required
            rows={5}
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            className="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white resize-y"
          />
        </div>

        <div>
          <label htmlFor="captcha" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            What is {captcha.a} {captcha.op} {captcha.b}?
          </label>
          <input
            id="captcha"
            type="number"
            required
            value={captchaInput}
            onChange={(e) => setCaptchaInput(e.target.value)}
            className="w-32 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
          />
        </div>

        {errorMessage && (
          <p className="text-sm text-red-600 dark:text-red-400">{errorMessage}</p>
        )}

        <button
          type="submit"
          disabled={status === "sending"}
          className="inline-flex items-center justify-center px-6 py-2.5 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium text-sm hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          {status === "sending" ? "Sending..." : "Send Message"}
        </button>
      </form>
    </StaticPage>
  )
}
