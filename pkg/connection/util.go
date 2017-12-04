package connection

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/api/core/v1"
)

const (
	nodeIDAnnotation = "csi.volume.kubernetes.io/nodeid"
)

func SanitizeDriverName(driver string) string {
	re := regexp.MustCompile("[^a-zA-Z0-9-]")
	name := re.ReplaceAllString(driver, "-")
	if name[len(name)-1] == '-' {
		// name must not end with '-'
		name = name + "X"
	}
	return name
}

// getFinalizerName returns Attacher name suitable to be used as finalizer
func GetFinalizerName(driver string) string {
	return "external-attacher/" + SanitizeDriverName(driver)
}

func GetNodeID(driver string, node *v1.Node) (string, error) {
	nodeIDJSON, ok := node.Annotations[nodeIDAnnotation]
	if !ok {
		return "", fmt.Errorf("node %q has no NodeID annotation", node.Name)
	}

	var nodeIDs map[string]string
	if err := json.Unmarshal([]byte(nodeIDJSON), &nodeIDs); err != nil {
		return "", fmt.Errorf("cannot parse NodeID annotation on node %q: %s", node.Name, err)
	}
	nodeID, ok := nodeIDs[driver]
	if !ok {
		return "", fmt.Errorf("cannot find NodeID for driver %q for node %q", driver, node.Name)
	}

	return nodeID, nil
}

func GetVolumeCapabilities(pv *v1.PersistentVolume) (*csi.VolumeCapability, error) {
	m := map[v1.PersistentVolumeAccessMode]bool{}
	for _, mode := range pv.Spec.AccessModes {
		m[mode] = true
	}

	cap := &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{
				// TODO: get FsType from somewhere
				MountFlags: pv.Spec.MountOptions,
			},
		},
		AccessMode: &csi.VolumeCapability_AccessMode{},
	}

	// Translate array of modes into single VolumeCapability
	switch {
	case m[v1.ReadWriteMany]:
		// ReadWriteMany trumps everything, regardless what other modes are set
		cap.AccessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER

	case m[v1.ReadOnlyMany] && m[v1.ReadWriteOnce]:
		// This is no way how to translate this to CSI...
		return nil, fmt.Errorf("CSI does not support ReadOnlyMany and ReadWriteOnce on the same PersistentVolume")

	case m[v1.ReadOnlyMany]:
		// There is only ReadOnlyMany set
		cap.AccessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY

	case m[v1.ReadWriteOnce]:
		// There is only ReadWriteOnce set
		cap.AccessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER

	default:
		return nil, fmt.Errorf("unsupported AccessMode combination: %+v", pv.Spec.AccessModes)
	}
	return cap, nil
}

func GetVolumeHandle(pv *v1.PersistentVolume) (string, bool, error) {
	if pv.Spec.PersistentVolumeSource.CSI == nil {
		return "", false, fmt.Errorf("persistent volume does not contain CSI volume source")
	}
	fmt.Printf("PV %+v",pv)
	return pv.Spec.PersistentVolumeSource.CSI.VolumeHandle, pv.Spec.PersistentVolumeSource.CSI.ReadOnly, nil
	//return pv.Spec.Source.PersistentVolumeName, pv.Spec.PersistentVolumeSource.CSI.ReadOnly, nil
}
