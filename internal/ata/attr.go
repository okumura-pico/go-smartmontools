package ata

// attrFlags controls how an attribute is tagged.
type attrFlags uint8

const (
	attrFlagHDDOnly attrFlags = 1 << iota
	attrFlagSSDOnly
)

type attrDef struct {
	name  string
	flags attrFlags
}

// defaultAttrDefs maps attribute ID → (name, HDD/SSD restriction).
// Source: DEFAULT entry in drivedb.h.
var defaultAttrDefs = map[uint8]attrDef{
	1:   {"Raw_Read_Error_Rate", 0},
	2:   {"Throughput_Performance", 0},
	3:   {"Spin_Up_Time", 0},
	4:   {"Start_Stop_Count", 0},
	5:   {"Reallocated_Sector_Ct", 0},
	6:   {"Read_Channel_Margin", attrFlagHDDOnly},
	7:   {"Seek_Error_Rate", attrFlagHDDOnly},
	8:   {"Seek_Time_Performance", attrFlagHDDOnly},
	9:   {"Power_On_Hours", 0},
	10:  {"Spin_Retry_Count", attrFlagHDDOnly},
	11:  {"Calibration_Retry_Count", attrFlagHDDOnly},
	12:  {"Power_Cycle_Count", 0},
	13:  {"Read_Soft_Error_Rate", 0},
	22:  {"Helium_Level", attrFlagHDDOnly},
	23:  {"Helium_Condition_Lower", attrFlagHDDOnly},
	24:  {"Helium_Condition_Upper", attrFlagHDDOnly},
	175: {"Program_Fail_Count_Chip", attrFlagSSDOnly},
	176: {"Erase_Fail_Count_Chip", attrFlagSSDOnly},
	177: {"Wear_Leveling_Count", attrFlagSSDOnly},
	178: {"Used_Rsvd_Blk_Cnt_Chip", attrFlagSSDOnly},
	179: {"Used_Rsvd_Blk_Cnt_Tot", attrFlagSSDOnly},
	180: {"Unused_Rsvd_Blk_Cnt_Tot", attrFlagSSDOnly},
	181: {"Program_Fail_Cnt_Total", 0},
	182: {"Erase_Fail_Count_Total", attrFlagSSDOnly},
	183: {"Runtime_Bad_Block", 0},
	184: {"End-to-End_Error", 0},
	187: {"Reported_Uncorrect", 0},
	188: {"Command_Timeout", 0},
	189: {"High_Fly_Writes", attrFlagHDDOnly},
	190: {"Airflow_Temperature_Cel", 0},
	191: {"G-Sense_Error_Rate", attrFlagHDDOnly},
	192: {"Power-Off_Retract_Count", 0},
	193: {"Load_Cycle_Count", attrFlagHDDOnly},
	194: {"Temperature_Celsius", 0},
	195: {"Hardware_ECC_Recovered", 0},
	196: {"Reallocated_Event_Count", 0},
	197: {"Current_Pending_Sector", 0},
	198: {"Offline_Uncorrectable", 0},
	199: {"UDMA_CRC_Error_Count", 0},
	200: {"Multi_Zone_Error_Rate", attrFlagHDDOnly},
	201: {"Soft_Read_Error_Rate", attrFlagHDDOnly},
	202: {"Data_Address_Mark_Errs", attrFlagHDDOnly},
	203: {"Run_Out_Cancel", 0},
	204: {"Soft_ECC_Correction", 0},
	205: {"Thermal_Asperity_Rate", 0},
	206: {"Flying_Height", attrFlagHDDOnly},
	207: {"Spin_High_Current", attrFlagHDDOnly},
	208: {"Spin_Buzz", attrFlagHDDOnly},
	209: {"Offline_Seek_Performnce", attrFlagHDDOnly},
	220: {"Disk_Shift", attrFlagHDDOnly},
	221: {"G-Sense_Error_Rate", attrFlagHDDOnly},
	222: {"Loaded_Hours", attrFlagHDDOnly},
	223: {"Load_Retry_Count", attrFlagHDDOnly},
	224: {"Load_Friction", attrFlagHDDOnly},
	225: {"Load_Cycle_Count", attrFlagHDDOnly},
	226: {"Load-in_Time", attrFlagHDDOnly},
	227: {"Torq-amp_Count", attrFlagHDDOnly},
	228: {"Power-off_Retract_Count", 0},
	230: {"Head_Amplitude", attrFlagHDDOnly},
	231: {"Temperature_Celsius", attrFlagHDDOnly},
	232: {"Available_Reservd_Space", 0},
	233: {"Media_Wearout_Indicator", attrFlagSSDOnly},
	240: {"Head_Flying_Hours", attrFlagHDDOnly},
	241: {"Total_LBAs_Written", 0},
	242: {"Total_LBAs_Read", 0},
	250: {"Read_Error_Retry_Rate", 0},
	254: {"Free_Fall_Sensor", attrFlagHDDOnly},
}

// AttrName returns the default name for a SMART attribute ID.
// rpm > 1 means HDD, rpm == 1 means SSD, rpm == 0 means unknown.
func AttrName(id uint8, rpm int) string {
	def, ok := defaultAttrDefs[id]
	if !ok {
		return "Unknown_Attribute"
	}
	if def.flags&attrFlagHDDOnly != 0 && rpm == 1 {
		return "Unknown_SSD_Attribute"
	}
	if def.flags&attrFlagSSDOnly != 0 && rpm > 1 {
		return "Unknown_HDD_Attribute"
	}
	return def.name
}
