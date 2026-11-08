export function isPasswordLengthValid(password: string): boolean {
  const byteLength = new TextEncoder().encode(password).length
  return [...password].length >= 8 && byteLength <= 72
}
