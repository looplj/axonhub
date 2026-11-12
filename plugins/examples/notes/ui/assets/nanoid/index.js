

import { urlAlphabet } from './url-alphabet/index.js'

export { urlAlphabet }

export let random = bytes => crypto.getRandomValues(new Uint8Array(bytes))

export let customRandom = (alphabet, defaultSize, getRandom) => {
  let safeByteCutoff = 256 - (256 % alphabet.length)

  if (safeByteCutoff === 256) {
    let mask = alphabet.length - 1

    return (size = defaultSize) => {
      if (!size) return ''
      let id = ''
      while (true) {
        let bytes = getRandom(size)
        let j = (size |= 0)
        while (j--) {
          id += alphabet[bytes[j] & mask]
          if (id.length >= size) return id
        }
      }
    }
  }

  return (size = defaultSize) => {
    if (!size) return ''
    let id = ''
    let step = Math.ceil((1.6 * 256 * size) / safeByteCutoff)
    while (true) {
      let bytes = getRandom(step)
      let j = step
      while (j--) {
        if (bytes[j] < safeByteCutoff) {
          id += alphabet[bytes[j] % alphabet.length]
          if (id.length >= size) return id
        }
      }
    }
  }
}

export let customAlphabet = (alphabet, size = 21) =>
  customRandom(alphabet, size | 0, random)

export let nanoid = (size = 21) => {
  var id = ''
  var bytes = crypto.getRandomValues(new Uint8Array((size |= 0)))
  while (size--) {
    id += urlAlphabet[bytes[size] & 63]
  }
  return id
}
