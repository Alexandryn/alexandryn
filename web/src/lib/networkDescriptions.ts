export function getReachabilityDescription(reachability: string): string {
  switch (reachability) {
    case 'loopback':
    case 'local_only':
      return 'Alexandryn is only accessible from this computer.'
    case 'private':
    case 'lan':
    case 'local_network':
      return 'Accessible from devices on your local network.'
    case 'public':
    case 'internet':
      return 'Accessible from the internet.'
    default:
      return reachability
  }
}

export function getTLSDescription(tlsMode: string): string {
  switch (tlsMode) {
    case 'none':
      return 'Traffic on your local network is unencrypted. Anyone with access to your Wi-Fi or router can see the books you read and the pages you view, unless you are using a reverse proxy that provides TLS.'
    case 'static':
      return 'Encrypted with a custom TLS certificate.'
    case 'acme':
      return "Encrypted with an automatic Let's Encrypt TLS certificate."
    default:
      return tlsMode
  }
}
