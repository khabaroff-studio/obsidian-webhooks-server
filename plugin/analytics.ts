import posthog from 'posthog-js'

export class Analytics {
    private enabled: boolean = false

    init(apiKey: string, host: string) {
        if (!apiKey) {
            return
        }

        posthog.init(apiKey, {
            api_host: host,
            autocapture: false,
            capture_pageview: false,
        })

        this.enabled = true
    }

    setUser(emailHash: string) {
        if (!this.enabled) return
        posthog.identify(`email_${emailHash}`)
    }

    trackEvent(event: string, properties?: Record<string, any>) {
        if (!this.enabled) return
        posthog.capture(event, properties)
    }
}

export const analytics = new Analytics()
