package domain

import (
	"os"
	"time"
)

type FileType string

const (
	TypeRegular   FileType = "arquivo_regular"
	TypeDirectory FileType = "diretorio"
	TypeSymlink   FileType = "link_simbolico"
	TypeSocket    FileType = "socket"
	TypeFIFO      FileType = "fifo"
	TypeBlock     FileType = "dispositivo_bloco"
	TypeCharacter FileType = "dispositivo_caractere"
	TypeUnknown   FileType = "desconhecido"
)

type PermissionInfo struct {
	NumericMode, SymbolicMode              string
	OwnerRead, OwnerWrite, OwnerExecute    bool
	GroupRead, GroupWrite, GroupExecute    bool
	OthersRead, OthersWrite, OthersExecute bool
	SUID, SGID, Sticky                     bool
}

type UserIdentity struct{ Name, UID string }
type GroupIdentity struct{ Name, GID string }

type FilesystemInfo struct {
	DeviceID uint64
	ReadOnly bool
}

type FileMetadata struct {
	Path, ResolvedPath, Name string
	Type                     FileType
	Size                     int64
	Mode                     PermissionInfo
	Owner                    UserIdentity
	Group                    GroupIdentity
	ModifiedAt               time.Time
	AccessedAt, ChangedAt    *time.Time
	Inode                    uint64
	IsSymlink                bool
	SymlinkTarget            string
	SymlinkTargetExists      *bool
	Filesystem               FilesystemInfo
	ReadOnlyFilesystem       bool
}

type DirectoryEntry struct {
	Metadata  FileMetadata
	ErrorCode string
}

type PrivilegeInfo struct {
	RealUID, EffectiveUID, RealGID, EffectiveGID int
	User                                         string
	Groups                                       []string
	IsRoot                                       bool
	CapabilitiesHex                              string
	HasRelevantCapabilities                      bool
	Mode                                         string
}

type Explanation struct {
	Scope string
	Bits  string
	Codes []string
}

type ChangeRequest struct {
	TargetPath, ResolvedPath                                           string
	CurrentMetadata                                                    FileMetadata
	ProposedMode                                                       *os.FileMode
	ProposedOwner                                                      *int
	ProposedGroup                                                      *int
	Recursive, FollowSymlinks, RequiresRoot, RequiresTypedConfirmation bool
}

type AuditRecord struct {
	Timestamp       time.Time `json:"timestamp"`
	OperatorUser    string    `json:"operator_user"`
	OperatorUID     int       `json:"operator_uid"`
	TargetPath      string    `json:"target_path"`
	TargetType      FileType  `json:"target_type"`
	Operation       string    `json:"operation"`
	PreviousMode    string    `json:"previous_mode"`
	NewMode         string    `json:"new_mode"`
	PreviousOwner   string    `json:"previous_owner"`
	PreviousGroup   string    `json:"previous_group"`
	Result          string    `json:"result"`
	Error           string    `json:"error,omitempty"`
	Recursive       bool      `json:"recursive"`
	SymlinkDetected bool      `json:"symlink_detected"`
}
