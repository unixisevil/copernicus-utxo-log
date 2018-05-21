package script

import (
	"encoding/binary"
	"github.com/pkg/errors"
	"bytes"
	"io"
	"github.com/btcboost/copernicus/model/opcodes"
	"github.com/btcboost/copernicus/util"
)
const (
	MaxMessagePayload = 32*1024*1024
)

const (
	DefaultSize = 28

	// MaxPubKeysPerMultiSig :  maximum number of public keys per multiSig
	MaxPubKeysPerMultiSig = 20

	// LockTimeThreshold threshold for nLockTime: below this value it is interpreted as block number,
	// otherwise as UNIX timestamp. Threshold is Tue Nov 5 00:53:20 1985 UTC
	LockTimeThreshold = 500000000

	// SequenceFinal setting sequence to this value for every input in a transaction
	// disables nLockTime.
	SequenceFinal = 0xffffffff

	MaxScriptSize        = 10000
	MaxScriptElementSize = 520
	MaxScriptOpCodes     = 201
	MaxOpsPerScript      = 201
)

const (
	// SequenceLockTimeDisableFlag below flags apply in the context of BIP 68*/
	// If this flag set, CTxIn::nSequence is NOT interpreted as a
	// relative lock-time. */
	SequenceLockTimeDisableFlag = 1 << 31

	// SequenceLockTimeTypeFlag if CTxIn::nSequence encodes a relative lock-time and this flag
	// is set, the relative lock-time has units of 512 seconds,
	// otherwise it specifies blocks with a granularity of 1.
	SequenceLockTimeTypeFlag = 1 << 22

	// SequenceLockTimeMask if CTxIn::nSequence encodes a relative lock-time, this mask is
	// applied to extract that lock-time from the sequence field.
	SequenceLockTimeMask = 0x0000ffff

	// SequenceLockTimeQranularity in order to use the same number of bits to encode roughly the
	// same wall-clock duration, and because blocks are naturally
	// limited to occur every 600s on average, the minimum granularity
	// for time-based relative lock-time is fixed at 512 seconds.
	// Converting from CTxIn::nSequence to seconds is performed by
	// multiplying by 512 = 2^9, or equivalently shifting up by
	// 9 bits.
	SequenceLockTimeGranularity = 9
)

/** Script verification flags */
const (
	ScriptVerifyNone = 0

	// Evaluate P2SH subscripts (softfork safe, BIP16).
	ScriptVerifyP2SH = (1 << 0)

	// Passing a non-strict-DER signature or one with undefined hashtype to a
	// checksig operation causes script failure. Evaluating a pubkey that is not
	// (0x04 + 64 bytes) or (0x02 or 0x03 + 32 bytes) by checksig causes script
	// failure.
	ScriptVerifyStrictEnc = (1 << 1)

	// Passing a non-strict-DER signature to a checksig operation causes script
	// failure (softfork safe, BIP62 rule 1)
	ScriptVerifyDersig = (1 << 2)

	// Passing a non-strict-DER signature or one with S > order/2 to a checksig
	// operation causes script failure
	// (softfork safe, BIP62 rule 5).
	ScriptVerifyLowS = (1 << 3)

	// verify dummy stack item consumed by CHECKMULTISIG is of zero-length
	// (softfork safe, BIP62 rule 7).
	ScriptVerifyNullDummy = (1 << 4)

	// Using a non-push operator in the scriptSig causes script failure
	// (softfork safe, BIP62 rule 2).
	ScriptVerifySigPushOnly = (1 << 5)

	// Require minimal encodings for all push operations (OP_0... OP_16,
	// OP_1NEGATE where possible, direct pushes up to 75 bytes, OP_PUSHDATA up
	// to 255 bytes, OP_PUSHDATA2 for anything larger). Evaluating any other
	// push causes the script to fail (BIP62 rule 3). In addition, whenever a
	// stack element is interpreted as a number, it must be of minimal length
	// (BIP62 rule 4).
	// (softfork safe)
	ScriptVerifyMinmalData = (1 << 6)

	// Discourage use of Nops reserved for upgrades (NOP1-10)
	//
	// Provided so that nodes can avoid accepting or mining transactions
	// containing executed NOP's whose meaning may change after a soft-fork,
	// thus rendering the script invalid; with this flag set executing
	// discouraged NOPs fails the script. This verification flag will never be a
	// mandatory flag applied to scripts in a block. NOPs that are not executed,
	// e.g.  within an unexecuted IF ENDIF block, are *not* rejected.
	ScriptVerifyDiscourageUpgradableNops = (1 << 7)

	// Require that only a single stack element remains after evaluation. This
	// changes the success criterion from "At least one stack element must
	// remain, and when interpreted as a boolean, it must be true" to "Exactly
	// one stack element must remain, and when interpreted as a boolean, it must
	// be true".
	// (softfork safe, BIP62 rule 6)
	// Note: CLEANSTACK should never be used without P2SH or WITNESS.
	ScriptVerifyCleanStack = (1 << 8)

	// Verify CHECKLOCKTIMEVERIFY
	//
	// See BIP65 for details.
	ScriptVerifyCheckLockTimeVerify = (1 << 9)

	// support CHECKSEQUENCEVERIFY opcode
	//
	// See BIP112 for details
	ScriptVerifyCheckSequenceVerify = (1 << 10)

	// Making v1-v16 witness program non-standard
	//
	ScriptVerifyDiscourageUpgradableWitnessProgram = (1 << 12)

	// Segwit script only: Require the argument of OP_IF/NOTIF to be exactly
	// 0x01 or empty vector
	//
	ScriptVerifyMinimalIf = (1 << 13)

	// Signature(s) must be empty vector if an CHECK(MULTI)SIG operation failed
	//
	ScriptVerifyNullFail = (1 << 14)

	// Public keys in scripts must be compressed
	//
	ScriptVerifyCompressedPubkeyType = (1 << 15)

	// Do we accept signature using SIGHASH_FORKID
	//
	ScriptEnableSigHashForkId = (1 << 16)

	// Do we accept activate replay protection using a different fork id.
	//
	ScriptEnableReplayProtection = (1 << 17)

	// Enable new opcodes.
	//
	ScriptEnableMonolithOpcodes = (1 << 18)
)

const (
	ScriptNonStandard = iota
	// 'standard' transaction types:
	ScriptPubkey
	ScriptPubkeyHash
	ScriptHash
	ScriptMultiSig
	ScriptNullData

	MaxOpReturnRelay uint = 83
	MaxOpReturnRelayLarge uint = 223
)
const (
	// MandatoryScriptVerifyFlags mandatory script verification flags that all new blocks must comply with for
	// them to be valid. (but old blocks may not comply with) Currently just P2SH,
	// but in the future other flags may be added, such as a soft-fork to enforce
	// strict DER encoding.
	//
	// Failing one of these tests may trigger a DoS ban - see CheckInputs() for
	// details.
	MandatoryScriptVerifyFlags uint =
	ScriptVerifyP2SH | ScriptVerifyStrictEnc |
		ScriptEnableSigHashForkId | ScriptVerifyLowS | ScriptVerifyNullFail

	/*StandardScriptVerifyFlags standard script verification flags that standard transactions will comply
	 * with. However scripts violating these flags may still be present in valid
	 * blocks and we must accept those blocks.
	 */
	StandardScriptVerifyFlags uint = MandatoryScriptVerifyFlags | ScriptVerifyDersig |
	ScriptVerifyMinmalData | ScriptVerifyNullDummy |
	ScriptVerifyDiscourageUpgradableNops | ScriptVerifyCleanStack |
	ScriptVerifyNullFail | ScriptVerifyCheckLockTimeVerify |
	ScriptVerifyCheckSequenceVerify | ScriptVerifyLowS |
	ScriptVerifyDiscourageUpgradableWitnessProgram

	/*StandardNotMandatoryVerifyFlags for convenience, standard but not mandatory verify flags. */
	StandardNotMandatoryVerifyFlags uint= StandardScriptVerifyFlags & (^MandatoryScriptVerifyFlags)
)

type Script struct {
	data          []byte
	ParsedOpCodes []opcodes.ParsedOpCode
}

func (s *Script) Serialize(io io.Writer) (err error) {
	return util.WriteVarBytes(io, s.data)
}

func (s *Script) UnSerialize(io io.Reader) (script *Script, err error) {
	bytes, err := ReadScript(io, MaxMessagePayload, "tx input signature script")
	if err != nil {
		return nil, err
	}

	return NewScriptRaw(bytes), err
}

func NewScriptRaw(bytes []byte) *Script {
	script := Script{data: bytes}
	if script.convertOPS() != nil {
		return nil
	}
	return &script
}

func NewScriptOps(parsedOpCodes []opcodes.ParsedOpCode) *Script {
	script := Script{ParsedOpCodes: parsedOpCodes}
	script.convertRaw()
	return &script
}

func (script *Script) convertRaw() {
	script.data = make([]byte, 0)
	for _, e := range script.ParsedOpCodes {
		script.data = append(script.data, e.OpValue)
		if e.OpValue == opcodes.OP_PUSHDATA1 {
			script.data = append(script.data, byte(e.Length))
		} else if e.OpValue == opcodes.OP_PUSHDATA2 {
			b := make([]byte, 2)
			binary.LittleEndian.PutUint16(b, uint16(e.Length))
			script.data = append(script.data, b...)
		} else if e.OpValue == opcodes.OP_PUSHDATA4 {
			b := make([]byte, 4)
			binary.LittleEndian.PutUint32(b, uint32(e.Length))
			script.data = append(script.data, b...)
		} else {
			if e.OpValue < opcodes.OP_PUSHDATA1 && e.Length > 0 {
				script.data = append(script.data, byte(e.Length))
				script.data = append(script.data, e.Data...)
			}
		}

	}

}

func (script *Script) GetData() []byte {
	retData := make([]byte, len(script.data))

	return append(retData, script.data...)
}

func (script *Script) convertOPS() error {
	script.ParsedOpCodes = make([]opcodes.ParsedOpCode, 0)
	scriptLen := len(script.data)

	for i := 0; i < scriptLen; i++ {
		var nSize int
		opcode := script.data[i]
		parsedopCode := opcodes.ParsedOpCode{OpValue: opcode}

		if opcode < opcodes.OP_PUSHDATA1 {
			nSize = int(opcode)
			if scriptLen - i < nSize {
				return errors.New("OP has no enough data")
			}
			parsedopCode.Data = script.data[i + 1: i + 1 + nSize]
		} else if opcode == opcodes.OP_PUSHDATA1 {
			if scriptLen - i < 1 {
				return errors.New("OP_PUSHDATA1 has no enough data")
			}

			nSize = int(script.data[i + 1])
			if scriptLen - i - 1 < nSize {
				return errors.New("OP_PUSHDATA1 has no enough data")
			}
			parsedopCode.Data = script.data[i + 2: i + 2 + nSize]
			i++
		} else if opcode == opcodes.OP_PUSHDATA2 {
			if scriptLen - i < 2 {
				return errors.New("OP_PUSHDATA2 has no enough data")
			}
			nSize = int(binary.LittleEndian.Uint16(script.data[i + 1: i + 3]))
			if scriptLen - i - 3 < nSize {
				return errors.New("OP_PUSHDATA2 has no enough data")
			}
			parsedopCode.Data = script.data[i + 3: i + 3 + nSize]
			i += 2
		} else if opcode == opcodes.OP_PUSHDATA4 {
			if scriptLen - i < 4 {
				return errors.New("OP_PUSHDATA4 has no enough data")

			}
			nSize = int(binary.LittleEndian.Uint32(script.data[i + 1: i + 5]))
			parsedopCode.Data = script.data[i + 5: i + 5 + nSize]
			i += 4
		}
		if scriptLen - i < 0 || (scriptLen - i) < nSize {
			return errors.New("size is wrong")

		}
		parsedopCode.Length = nSize

		script.ParsedOpCodes = append(script.ParsedOpCodes, parsedopCode)

		i += nSize
	}
	return nil
}

func (script *Script)RemoveOpcodeByData(data []byte) *Script{
	parsedOpCodes := make([]opcodes.ParsedOpCode, len(script.ParsedOpCodes))
	for _, e := range script.ParsedOpCodes {
		if bytes.Contains(e.Data, data) {
			continue
		}
		parsedOpCodes = append(parsedOpCodes, e)
	}
	return NewScriptOps(parsedOpCodes)
}

func (script *Script)RemoveOpcode(code byte) *Script{
	parsedOpCodes := make([]opcodes.ParsedOpCode, len(script.ParsedOpCodes))
	for _, e := range script.ParsedOpCodes {
		if e.OpValue == code {
			continue
		}
		parsedOpCodes = append(parsedOpCodes, e)
	}
	return NewScriptOps(parsedOpCodes)
}

func ReadScript(reader io.Reader, maxAllowed uint32, fieldName string) (script []byte, err error) {
	count, err := util.ReadVarInt(reader)
	if err != nil {
		return
	}
	if count > uint64(maxAllowed) {
		err = errors.Errorf("readScript %s is larger than the max allowed size [count %d,max %d]", fieldName, count, maxAllowed)
		return
	}
	//buf := scriptPool.Borrow(count)
	_, err = io.ReadFull(reader, script)
	if err != nil {
		//scriptPool.Return(buf)
		return
	}
	return script, nil

}

func (script *Script) ExtractDestinations() (sType int, address [][]byte, sigCountRequired int, err error) {
	return
}
/*
func (script *Script) IsCommitment(data []byte) bool {
	if len(data) > 64 || script.Size() != len(data)+2 {
		return false
	}

<<<<<<< HEAD
	if script.data[0] != OP_RETURN || int(script.data[1]) != len(data) {
=======
	if script.byteCodes[0] != OP_RETURN || int(script.byteCodes[1]) != len(data) {
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
		return false
	}

	for i := 0; i < len(data); i++ {
<<<<<<< HEAD
		if script.data[i+2] != data[i] {
=======
		if script.byteCodes[i+2] != data[i] {
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
			return false
		}
	}

	return true
}
*/

func BytesToBool(bytes []byte) bool {
	bytesLen := len(bytes)
	if bytesLen == 0 {
		return false
	}
	for i, e := range bytes {
		if uint8(e) != 0 {
			if i == bytesLen - 1 && e == 0x80 {
				return false
			}
			return true
		}
	}
	return false
}

func (script *Script) CheckScriptPubKey() (succeed bool, pubKeyType int) {
	//p2sh scriptPubKey
	if script.IsPayToScriptHash() {
		return true, ScriptHash
	}
	// Provably prunable, data-carrying output
	//
	// So long as script passes the IsUnspendable() test and all but the first
	// byte passes the IsPushOnly() test we don't care what exactly is in the
	// script.
	len := len(script.ParsedOpCodes)
	if len == 0 {
		return false, ScriptNonStandard
	}
	parsedOpCode0 := script.ParsedOpCodes[0]
	opValue0 := parsedOpCode0.OpValue

	// OP_RETURN
	if len == 1 {
		if parsedOpCode0.OpValue == opcodes.OP_RETURN {
			return true, ScriptNullData
		}
		return false, ScriptNonStandard
	}

	// OP_RETURN and DATA
	if parsedOpCode0.OpValue == opcodes.OP_RETURN {
		tempScript := NewScriptOps(script.ParsedOpCodes[1:])
		if tempScript.IsPushOnly() {
			return true, ScriptNullData
		}
		return false, ScriptNonStandard
	}

	//PUBKEY OP_CHECKSIG
	if len == 2 {
		if opValue0 > opcodes.OP_PUSHDATA4 || parsedOpCode0.Length < 33 ||
			parsedOpCode0.Length > 65 || script.ParsedOpCodes[1].OpValue != opcodes.OP_CHECKSIG {
			return false, ScriptNonStandard
		}
		return true, ScriptPubkey
	}

	//OP_DUP OP_HASH160 OP_PUBKEYHASH OP_EQUALVERIFY OP_CHECKSIG
	if opValue0 == opcodes.OP_DUP {
		if script.ParsedOpCodes[1].OpValue != opcodes.OP_HASH160 ||
			script.ParsedOpCodes[2].OpValue != opcodes.OP_PUBKEYHASH ||
			script.ParsedOpCodes[2].Length != 20 ||
			script.ParsedOpCodes[3].OpValue != opcodes.OP_EQUALVERIFY ||
			script.ParsedOpCodes[4].OpValue != opcodes.OP_CHECKSIG {
			return false, ScriptNonStandard
		}
		return true, ScriptPubkeyHash
	}

	//m pubkey1 pubkey2...pubkeyn n OP_CHECKMULTISIG
	if opValue0 == opcodes.OP_0 || (opValue0 >= opcodes.OP_1 && opValue0 <= opcodes.OP_16) {
		opM, _ := DecodeOPN(opValue0)
		i := 1
		pubKeyCount := 0
		for script.ParsedOpCodes[i].Length >= 33 && script.ParsedOpCodes[i].Length <= 65 {
			pubKeyCount++
			i++
		}
		opValueI := script.ParsedOpCodes[i].OpValue
		if opValueI == opcodes.OP_0 || (opValue0 >= opcodes.OP_1 && opValue0 <= opcodes.OP_16) {
			opN, _ := DecodeOPN(opValueI)
			// Support up to x-of-3 multisig txns as standard
			if opM < 1 || opN < 1 || opN > 3 || opM > opN || opN != pubKeyCount {
				return false, ScriptNonStandard
			}
			i++
		} else {
			return false, ScriptNonStandard
		}
		if script.ParsedOpCodes[i].OpValue != opcodes.OP_CHECKMULTISIG {
			return false, ScriptNonStandard
		}
		return true, ScriptMultiSig
	}

	return false, ScriptNonStandard
}

func (script *Script) CheckScriptSig() bool{
	if script.Size() > 1650 {
		//state.Dos(100, false, RejectInvalid, "bad-tx-input-script-size", false, "")
		return false
	}
	if !script.IsPushOnly() {
		//state.Dos(100, false, RejectInvalid, "bad-tx-input-script-not-pushonly", false, "")
		return false
	}

	return true
}

func (script *Script) IsPayToScriptHash() bool {
	size := len(script.data)
	return size == 23 &&
		script.data[0] == opcodes.OP_HASH160 &&
		script.data[1] == 0x14 &&
		script.data[22] == opcodes.OP_EQUAL
}

func (script *Script) IsUnspendable() bool {
	return script.Size() > 0 &&
		script.ParsedOpCodes[0].OpValue == opcodes.OP_RETURN ||
		script.Size() > MaxScriptSize
}
/*
func CheckMinimalPush(data []byte, opcode int32) bool {
	dataLen := len(data)
	if dataLen == 0 {
		// Could have used OP_0.
		return opcode == opcodes.OP_0
	}
	if dataLen == 1 && data[0] >= 1 && data[0] <= 16 {
		// Could have used OP_1 .. OP_16.
		return opcode == (opcodes.OP_1 + int32(data[0]-1))
	}
	if dataLen == 1 && data[0] == 0x81 {
		return opcode == opcodes.OP_1NEGATE
	}
	if dataLen <= 75 {
		// Could have used a direct push (opcode indicating number of byteCodes
		// pushed + those byteCodes).
		return opcode == int32(dataLen)
	}
	if dataLen <= 255 {
		// Could have used OP_PUSHDATA.
		return opcode == opcodes.OP_PUSHDATA1
	}
	if dataLen <= 65535 {
		// Could have used OP_PUSHDATA2.
		return opcode == opcodes.OP_PUSHDATA2
	}
	return true
}
*/
/*
func (script *Script) GetOp(index *int, opCode *byte, data *[]byte) bool {

	opcode := byte(OP_INVALIDOPCODE)
	tmpIndex := *index
	tmpData := make([]byte, 0)
	if tmpIndex >= script.Size() {
		return false
	}

	// Read instruction
	if script.Size() - tmpIndex < 1 {
		return false
	}

<<<<<<< HEAD
	opcode = script.data[tmpIndex]
=======
	opcode = script.byteCodes[tmpIndex]
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
	tmpIndex++

	// Immediate operand
	if opcode <= OP_PUSHDATA4 {
		nSize := 0
		if opcode < OP_PUSHDATA1 {
			nSize = int(opcode)
		} else if opcode == OP_PUSHDATA1 {
			if script.Size() - tmpIndex < 1 {
				return false
			}
<<<<<<< HEAD
			nSize = int(script.data[*index])
=======
			nSize = int(script.byteCodes[*index])
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
			tmpIndex++
		} else if opcode == OP_PUSHDATA2 {
			if script.Size() - tmpIndex < 2 {
				return false
			}
<<<<<<< HEAD
			nSize = int(binary.LittleEndian.Uint16(script.data[tmpIndex : tmpIndex+2]))
=======
			nSize = int(binary.LittleEndian.Uint16(script.byteCodes[tmpIndex : tmpIndex+2]))
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
			tmpIndex += 2
		} else if opcode == OP_PUSHDATA4 {
			if script.Size() - tmpIndex < 4 {
				return false
			}
<<<<<<< HEAD
			nSize = int(binary.LittleEndian.Uint32(script.data[tmpIndex : tmpIndex+4]))
=======
			nSize = int(binary.LittleEndian.Uint32(script.byteCodes[tmpIndex : tmpIndex+4]))
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
			tmpIndex += 4
		}
		if script.Size()-tmpIndex < 0 || script.Size()-tmpIndex < nSize {
			return false
		}
<<<<<<< HEAD
		tmpData = append(tmpData, script.data[tmpIndex:tmpIndex+nSize]...)
=======
		tmpData = append(tmpData, script.byteCodes[tmpIndex:tmpIndex+nSize]...)
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
		tmpIndex += nSize
	}

	*data = tmpData
	*opCode = opcode
	*index = tmpIndex
	return true
}*/

/*
func (script *Script) PushInt64(n int64) {

	if n == -1 || (n >= 1 && n <= 16) {
<<<<<<< HEAD
		script.data = append(script.data, byte(n+(OP_1-1)))
	} else if n == 0 {
		script.data = append(script.data, byte(OP_0))
	} else {
		scriptNum := NewCScriptNum(n)
		script.data = append(script.data, scriptNum.Serialize()...)
=======
		script.byteCodes = append(script.byteCodes, byte(n+(OP_1-1)))
	} else if n == 0 {
		script.byteCodes = append(script.byteCodes, byte(OP_0))
	} else {
		scriptNum := NewCScriptNum(n)
		script.byteCodes = append(script.byteCodes, scriptNum.Serialize()...)
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
	}
}

func (script *Script) PushOpCode(opcode int) error {
	if opcode < 0 || opcode > 0xff {
		return errors.New("push opcode failed :invalid opcode")
	}
<<<<<<< HEAD
	script.data = append(script.data, byte(opcode))
=======
	script.byteCodes = append(script.byteCodes, byte(opcode))
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
	return nil
}

func (script *Script) PushScriptNum(scriptNum *CScriptNum) {
<<<<<<< HEAD
	script.data = append(script.data, scriptNum.Serialize()...)
=======
	script.byteCodes = append(script.byteCodes, scriptNum.Serialize()...)
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
}

func (script *Script) PushData(data []byte) {
	dataLen := len(data)
	if dataLen < OP_PUSHDATA1 {
<<<<<<< HEAD
		script.data = append(script.data, byte(dataLen))
	} else if dataLen <= 0xff {
		script.data = append(script.data, OP_PUSHDATA1)
		script.data = append(script.data, byte(dataLen))
	} else if dataLen <= 0xffff {
		script.data = append(script.data, OP_PUSHDATA2)
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(dataLen))
		script.data = append(script.data, buf...)

	} else {
		script.data = append(script.data, OP_PUSHDATA4)
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(script.data, uint32(dataLen))
		script.data = append(script.data, buf...)
	}
	script.data = append(script.data, data...)
=======
		script.byteCodes = append(script.byteCodes, byte(dataLen))
	} else if dataLen <= 0xff {
		script.byteCodes = append(script.byteCodes, OP_PUSHDATA1)
		script.byteCodes = append(script.byteCodes, byte(dataLen))
	} else if dataLen <= 0xffff {
		script.byteCodes = append(script.byteCodes, OP_PUSHDATA2)
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(dataLen))
		script.byteCodes = append(script.byteCodes, buf...)

	} else {
		script.byteCodes = append(script.byteCodes, OP_PUSHDATA4)
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(script.byteCodes, uint32(dataLen))
		script.byteCodes = append(script.byteCodes, buf...)
	}
	script.byteCodes = append(script.byteCodes, data...)
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
}
*/
/*
func (script *Script) ParseScript() (stk []ParsedOpCode, err error) {
	stk = make([]ParsedOpCode, 0)
<<<<<<< HEAD
	scriptLen := len(script.data)

	for i := 0; i < scriptLen; i++ {
		var nSize int
		opcode := script.data[i]
=======
	scriptLen := len(script.byteCodes)

	for i := 0; i < scriptLen; i++ {
		var nSize int
		opcode := script.byteCodes[i]
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
		parsedopCode := ParsedOpCode{opValue: opcode}

		if opcode < OP_PUSHDATA1 {
			nSize = int(opcode)
<<<<<<< HEAD
			parsedopCode.data = script.data[i+1 : i+1+nSize]
=======
			parsedopCode.data = script.byteCodes[i+1 : i+1+nSize]
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7

		} else if opcode == OP_PUSHDATA1 {
			if scriptLen-i < 1 {
				err = errors.New("OP_PUSHDATA1 has no enough data")
				return
			}
<<<<<<< HEAD
			nSize = int(script.data[i+1])
			parsedopCode.data = script.data[i+2 : i+2+nSize]
=======
			nSize = int(script.byteCodes[i+1])
			parsedopCode.data = script.byteCodes[i+2 : i+2+nSize]
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
			i++

		} else if opcode == OP_PUSHDATA2 {
			if scriptLen-i < 2 {
				err = errors.New("OP_PUSHDATA2 has no enough data")
				return
			}
<<<<<<< HEAD
			nSize = int(binary.LittleEndian.Uint16(script.data[i+1 : i+3]))
			parsedopCode.data = script.data[i+3 : i+3+nSize]
=======
			nSize = int(binary.LittleEndian.Uint16(script.byteCodes[i+1 : i+3]))
			parsedopCode.data = script.byteCodes[i+3 : i+3+nSize]
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
			i += 2
		} else if opcode == OP_PUSHDATA4 {
			if scriptLen-i < 4 {
				err = errors.New("OP_PUSHDATA4 has no enough data")
				return
			}
<<<<<<< HEAD
			nSize = int(binary.LittleEndian.Uint32(script.data[i+1 : i+5]))
			parsedopCode.data = script.data[i+5 : i+5+nSize]
=======
			nSize = int(binary.LittleEndian.Uint32(script.byteCodes[i+1 : i+5]))
			parsedopCode.data = script.byteCodes[i+5 : i+5+nSize]
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
			i += 4
		}
		if scriptLen-i < 0 || (scriptLen-i) < nSize {
			err = errors.New("size is wrong")
			return
		}

		stk = append(stk, parsedopCode)
		i += nSize
	}

	return
}
*/
/*
func (script *Script) FindAndDelete(b *Script) (bool, error) {
	//orginalParseCodes, err := script.ParseScript()
	//if err != nil {
	//	return false, err
	//}
	//paramScript, err := b.ParseScript()
	//if err != nil {
	//	return false, err
	//}
	//script.data = make([]byte, 0)

	for i := 0; i < len(orginalParseCodes); i++ {
		isDelete := false
		parseCode := orginalParseCodes[i]
		for j := 0; j < len(paramScript); j++ {
			parseCodeOther := paramScript[j]
			if parseCode.opValue == parseCodeOther.opValue {
				isDelete = true
			}
		}
		if !isDelete {
<<<<<<< HEAD
			script.data = append(script.data, parseCode.opValue)
			script.data = append(script.data, parseCode.data...)
=======
			script.byteCodes = append(script.byteCodes, parseCode.opValue)
			script.byteCodes = append(script.byteCodes, parseCode.data...)
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
		}
	}

	return true, nil
}
*/
/*
func (script *Script) Find(opcode int) bool {
	//stk, err := script.ParseScript()
	//if err != nil {
	//	return false
	//}
	for _, ops := range script.ParsedOpCodes {
		if int(ops.opValue) == opcode {
			return true
		}
	}
	return false
}
*/

func (script *Script) IsPushOnly() bool {
	for _, ops := range script.ParsedOpCodes {
		if ops.OpValue > OP_16 {
			return false
		}
	}
	return true

}
/*
func (script *Script) GetSigOpCount() (int, error) {
	if !script.IsPayToScriptHash() {
		return script.GetSigOpCountWithAccurate(true)
	}
	stk, err := script.ParseScript()
	if err != nil {
		return 0, err
	}
	if len(stk) == 0 {
		return 0, nil
	}
	for i := 0; i < len(stk); i++ {
		opcode := stk[i].opValue
		if opcode == OP_16 {
			return 0, nil
		}
	}
	return script.GetSigOpCountWithAccurate(true)
}

func (script *Script) GetSigOpCountFor(scriptSig *Script) (int, error) {
	if !script.IsPayToScriptHash() {
		return script.GetSigOpCountWithAccurate(true)
	}

	// This is a pay-to-script-hash scriptPubKey;
	// get the last item that the scriptSig
	// pushes onto the stack:
	var n = 0
	stk, err := scriptSig.ParseScript()
	if err != nil {
		return n, err
	}

	data := make([]byte, 0)
	for i := 0; i < len(stk); i++ {
		var opcode *byte
		if !scriptSig.GetOp(&i, opcode, &data) {
			return 0, nil
		}

		if *opcode > OP_16 {
			return 0, nil
		}
	}

	subScript := NewScriptRaw(data)
	return subScript.GetSigOpCountWithAccurate(true)
}
*/
/*
func (script *Script) GetScriptByte() []byte {
	scriptByte := make([]byte, 0)
<<<<<<< HEAD
	scriptByte = append(scriptByte, script.data...)
=======
	scriptByte = append(scriptByte, script.byteCodes...)
>>>>>>> c094fa5c6f05ba4ae9dab8c6668ccf09996efbc7
	return scriptByte
}
*/
func (script *Script) GetSigOpCount(accurate bool) (int, error) {
	n := 0
	//stk, err := script.ParseScript()
	//if err != nil {
	//	return n, err
	//}
	var lastOpcode byte
	for _, e := range script.ParsedOpCodes {
		opcode := e.OpValue
		if opcode == opcodes.OP_CHECKSIG || opcode == opcodes.OP_CHECKSIGVERIFY {
			n++
		} else if opcode == opcodes.OP_CHECKMULTISIG || opcode == opcodes.OP_CHECKMULTISIGVERIFY {
			if accurate && lastOpcode >= opcodes.OP_1 && lastOpcode <= opcodes.OP_16 {
				opn, err := DecodeOPN(lastOpcode)
				if err != nil {
					return 0, err
				}
				n += opn
			} else {
				n += MaxPubKeysPerMultiSig
			}
		}
		lastOpcode = opcode
	}
	return n, nil
}

func (script *Script) GetP2SHSigOpCount() (int, error) {
	// This is a pay-to-script-hash scriptPubKey;
	// get the last item that the scriptSig
	// pushes onto the stack:
	for _, e := range script.ParsedOpCodes {
		opcode := e.OpValue
		if opcode > opcodes.OP_16 {
			return 0, nil
		}
	}
	lastOps := script.ParsedOpCodes[len(script.ParsedOpCodes) - 1]
	tempScript := NewScriptRaw(lastOps.Data)
	return tempScript.GetSigOpCount(true)

}


func EncodeOPN(n int) (int, error) {
	if n < 0 || n > 16 {
		return 0, errors.New("EncodeOPN n is out of bounds")
	}
	return opcodes.OP_1 + n - 1, nil
}

func DecodeOPN(opcode byte) (int, error) {
	if opcode < opcodes.OP_0 || opcode > opcodes.OP_16 {
		return 0, errors.New(" DecodeOPN opcode is out of bounds")
	}
	return int(opcode) - int(opcodes.OP_1 - 1), nil
}

func (script *Script) Size() int {
	return len(script.data)
}

func (script *Script) IsEqual(script2 *Script) bool {
	/*if script.Size() != script2.Size() {
		return false
	}*/

	return bytes.Equal(script.data, script2.data)
}
