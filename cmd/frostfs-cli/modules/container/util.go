package container

import (
	"errors"

	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/common"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/commonflags"
	cid "github.com/TrueCloudLab/frostfs-sdk-go/container/id"
	"github.com/TrueCloudLab/frostfs-sdk-go/session"
	"github.com/spf13/cobra"
)

const (
	attributeDelimiter = "="

	awaitTimeout = 120 // in seconds
)

var (
	errCreateTimeout  = errors.New("timeout: container has not been persisted on sidechain")
	errDeleteTimeout  = errors.New("timeout: container has not been removed from sidechain")
	errSetEACLTimeout = errors.New("timeout: EACL has not been persisted on sidechain")
)

func parseContainerID(cmd *cobra.Command) cid.ID {
	if containerID == "" {
		common.ExitOnErr(cmd, "", errors.New("container ID is not set"))
	}

	var id cid.ID
	err := id.DecodeString(containerID)
	common.ExitOnErr(cmd, "can't decode container ID value: %w", err)
	return id
}

// decodes session.Container from the file by path provided in
// commonflags.SessionToken flag. Returns nil if the path is not specified.
func getSession(cmd *cobra.Command) *session.Container {
	common.PrintVerbose(cmd, "Reading container session...")

	path, _ := cmd.Flags().GetString(commonflags.SessionToken)
	if path == "" {
		common.PrintVerbose(cmd, "Session not provided.")
		return nil
	}

	common.PrintVerbose(cmd, "Reading container session from the file [%s]...", path)

	var res session.Container

	err := common.ReadBinaryOrJSON(cmd, &res, path)
	common.ExitOnErr(cmd, "read container session: %v", err)

	common.PrintVerbose(cmd, "Session successfully read.")

	return &res
}
