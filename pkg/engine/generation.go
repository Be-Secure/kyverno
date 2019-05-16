package engine

import (
	"fmt"
	"log"

	kubeClient "github.com/nirmata/kube-policy/kubeclient"
	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/engine/mutation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GenerationResponse struct {
	Generator *kubepolicy.Generation
	Namespace string
}

// Generate should be called to process generate rules on the resource
// TODO: extend kubeclient(will change to dynamic client) to create resources
func Generate(policy kubepolicy.Policy, rawResource []byte, kubeClient *kubeClient.KubeClient, gvk metav1.GroupVersionKind) {
	// configMapGenerator and secretGenerator can be applied only to namespaces
	if gvk.Kind != "Namespace" {
		return
	}

	for i, rule := range policy.Spec.Rules {

		// Checks for preconditions
		// TODO: Rework PolicyEngine interface that it receives not a policy, but mutation object for
		// Mutate, validation for Validate and so on. It will allow to bring this checks outside of PolicyEngine
		// to common part as far as they present for all: mutation, validation, generation

		err := rule.Validate()
		if err != nil {
			log.Printf("Rule has invalid structure: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		ok, err := mutation.ResourceMeetsRules(rawResource, rule.ResourceDescription, gvk)
		if err != nil {
			log.Printf("Rule has invalid data: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		if !ok {
			log.Printf("Rule is not applicable to the request: rule name = %s in policy %s \n", rule.Name, policy.ObjectMeta.Name)
			continue
		}

		err = applyRuleGenerator(rawResource, rule.Generation, kubeClient)
		if err != nil {
			log.Printf("Failed to apply rule generator: %v", err)
		}
	}
}

// Applies "configMapGenerator" and "secretGenerator" described in PolicyRule
// TODO: plan to support all kinds of generator
func applyRuleGenerator(rawResource []byte, generator *kubepolicy.Generation, kubeClient *kubeClient.KubeClient) error {
	if generator == nil {
		return nil
	}

	err := generator.Validate()
	if err != nil {
		return fmt.Errorf("Generator for '%s/%s' is invalid: %s", generator.Kind, generator.Name, err)
	}

	namespace := mutation.ParseNameFromObject(rawResource)
	switch generator.Kind {
	case "ConfigMap":
		err = kubeClient.GenerateConfigMap(*generator, namespace)
	case "Secret":
		err = kubeClient.GenerateSecret(*generator, namespace)
	default:
		err = fmt.Errorf("Unsupported config Kind '%s'", generator.Kind)
	}

	if err != nil {
		return fmt.Errorf("Unable to apply generator for %s '%s/%s' : %v", generator.Kind, namespace, generator.Name, err)
	}

	log.Printf("Successfully applied generator %s/%s", generator.Kind, generator.Name)
	return nil
}
