package provisioning

import "github.com/jmcleod/edgefabric/internal/domain"

// PipelineFor returns the ordered step sequence for a provisioning action.
func PipelineFor(action domain.ProvisioningAction) []domain.ProvisioningStep {
	switch action {
	case domain.ProvisionActionEnroll:
		return []domain.ProvisioningStep{
			domain.StepValidateSSH,
			domain.StepUploadBinary,
			domain.StepWriteConfig,
			domain.StepInstallSystemd,
			domain.StepStartService,
			domain.StepGenerateWGKeys,
			domain.StepConfigureWireGuard,
			domain.StepWaitEnrollment,
			domain.StepVerifyOnline,
		}
	case domain.ProvisionActionUpgrade:
		return []domain.ProvisioningStep{
			domain.StepValidateSSH,
			domain.StepUploadBinary,
			domain.StepSendCommand, // restart
			domain.StepVerifyOnline,
		}
	case domain.ProvisionActionStart:
		return []domain.ProvisioningStep{
			domain.StepSendCommand, // start
		}
	case domain.ProvisionActionStop:
		return []domain.ProvisioningStep{
			domain.StepSendCommand, // stop
		}
	case domain.ProvisionActionRestart:
		return []domain.ProvisioningStep{
			domain.StepSendCommand, // restart
		}
	case domain.ProvisionActionReprovision:
		return []domain.ProvisioningStep{
			domain.StepValidateSSH,
			domain.StepUploadBinary,
			domain.StepWriteConfig,
			domain.StepInstallSystemd,
			domain.StepStartService,
			domain.StepGenerateWGKeys,
			domain.StepConfigureWireGuard,
			domain.StepWaitEnrollment,
			domain.StepVerifyOnline,
		}
	case domain.ProvisionActionDecommission:
		return []domain.ProvisioningStep{
			domain.StepSendCommand, // stop
			domain.StepCleanup,
		}
	default:
		return nil
	}
}
