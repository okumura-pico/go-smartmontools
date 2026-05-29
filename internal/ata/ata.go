// Package ata provides ATA SMART data parsing for smartctl --all.
package ata

// ATA SMART subcommand codes (feature register value when using ATA_SMART_CMD).
const (
	SmartReadLog        = 0xd5
	SmartReadLogAddrError    = 0x01
	SmartReadLogAddrSelfTest = 0x06
	SmartReadLogAddrSelective = 0x09
)

// Number of vendor-specific SMART attributes per the ATA spec.
const NumAttributes = 30

// Attribute flag bits (from SFF 8035i Revision 2).
const (
	AttrFlagPrefailure     = 0x0001
	AttrFlagOnline         = 0x0002
	AttrFlagPerformance    = 0x0004
	AttrFlagErrorRate      = 0x0008
	AttrFlagEventCount     = 0x0010
	AttrFlagSelfPreserving = 0x0020
)
