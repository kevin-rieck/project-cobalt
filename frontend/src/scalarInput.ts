const supportedScalarDataTypes = ['Boolean', 'SByte', 'Int16', 'Int32', 'Int64', 'Byte', 'UInt16', 'UInt32', 'UInt64', 'Float', 'Double', 'String']
const decimalFloatPattern = /^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$/

export function isSupportedScalarDataType(dataType: string) {
  return supportedScalarDataTypes.includes(dataType.trim())
}

export function parseScalarInputError(dataType: string, target: string, label: string) {
  const trimmed = target.trim()
  if (!dataType || !isSupportedScalarDataType(dataType)) return ''
  if (dataType === 'String') return ''
  if (!trimmed) return label === 'Target Value' ? 'Enter a Target Value.' : `Enter a value for ${label}.`
  if (dataType === 'Boolean') return ['true', 'false', '1', '0', 'on', 'off', 'yes', 'no'].includes(trimmed.toLowerCase()) ? '' : `${label} must parse as Boolean.`
  if (dataType === 'Float' || dataType === 'Double') {
    if (!decimalFloatPattern.test(trimmed)) return `${label} must parse as ${dataType}.`
    const parsed = Number(trimmed)
    const significand = trimmed.split(/[eE]/, 1)[0]
    const representsNonZero = /[1-9]/.test(significand)
    const represented = dataType === 'Float' ? Math.fround(parsed) : parsed
    const inRange = Number.isFinite(represented) && !(representsNonZero && represented === 0)
    return inRange ? '' : `${label} must parse as ${dataType}.`
  }
  if (!/^[+]?\d+$/.test(trimmed) && ['Byte', 'UInt16', 'UInt32', 'UInt64'].includes(dataType)) return `${label} must be an unsigned plain decimal integer for ${dataType}.`
  if (!/^[+-]?\d+$/.test(trimmed)) return `${label} must be a plain decimal integer for ${dataType}.`
  const value = BigInt(trimmed)
  const ranges: Record<string, [bigint, bigint]> = {
    SByte: [BigInt('-128'), BigInt('127')], Int16: [BigInt('-32768'), BigInt('32767')], Int32: [BigInt('-2147483648'), BigInt('2147483647')], Int64: [BigInt('-9223372036854775808'), BigInt('9223372036854775807')],
    Byte: [BigInt('0'), BigInt('255')], UInt16: [BigInt('0'), BigInt('65535')], UInt32: [BigInt('0'), BigInt('4294967295')], UInt64: [BigInt('0'), BigInt('18446744073709551615')]
  }
  const [min, max] = ranges[dataType]
  return value < min || value > max ? `${label} is outside ${dataType} range.` : ''
}
