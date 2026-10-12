declare module 'zmodem.js/src/zmodem_browser' {
  export interface Offer {
    accept(): Promise<Array<Uint8Array<ArrayBuffer>>>
    get_details(): { name: string }
  }

  export interface Session {
    type: 'send' | 'receive'
    abort(): void
    close(): Promise<void>
    on(event: 'offer', handler: (offer: Offer) => void): void
    on(event: 'session_end', handler: () => void): void
    start(): void
  }

  interface Detection {
    confirm(): Session
  }

  interface SentryOptions {
    to_terminal(octets: number[]): void
    sender(octets: number[]): void
    on_detect(detection: Detection): void
    on_retract(): void
  }

  interface Sentry {
    consume(octets: ArrayBuffer | Uint8Array): void
  }

  const Zmodem: {
    Sentry: new (options: SentryOptions) => Sentry
    Browser: {
      send_files(session: Session, files: FileList | File[]): Promise<void>
      save_to_disk(payloads: Uint8Array[], name: string): void
    }
  }

  export default Zmodem
}
