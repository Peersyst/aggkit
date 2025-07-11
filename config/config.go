package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	jRPC "github.com/0xPolygon/cdk-rpc/rpc"
	"github.com/agglayer/aggkit/aggoracle"
	aggsendercfg "github.com/agglayer/aggkit/aggsender/config"
	"github.com/agglayer/aggkit/aggsender/prover"
	"github.com/agglayer/aggkit/bridgesync"
	"github.com/agglayer/aggkit/common"
	"github.com/agglayer/aggkit/l1infotreesync"
	"github.com/agglayer/aggkit/lastgersync"
	"github.com/agglayer/aggkit/log"
	"github.com/agglayer/aggkit/pprof"
	"github.com/agglayer/aggkit/prometheus"
	"github.com/agglayer/aggkit/reorgdetector"
	"github.com/mitchellh/mapstructure"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
)

const (
	// FlagCfg is the flag for cfg.
	FlagCfg = "cfg"
	// FlagComponents is the flag for components.
	FlagComponents = "components"
	// FlagSaveConfigPath is the flag to save the final configuration file
	FlagSaveConfigPath = "save-config-path"
	// FlagDisableDefaultConfigVars is the flag to force all variables to be set on config-files
	FlagDisableDefaultConfigVars = "disable-default-config-vars"
	// FlagAllowDeprecatedFields is the flag to allow deprecated fields
	FlagAllowDeprecatedFields = "allow-deprecated-fields"

	EnvVarPrefix       = "CDK"
	ConfigType         = "toml"
	SaveConfigFileName = "aggkit_config.toml"

	DefaultCreationFilePermissions = os.FileMode(0600)

	bridgeAddrSetOnWrongSection = "Bridge contract address must be set in the root of " +
		"config file as polygonBridgeAddr."
	l2URLHint                = "Use L2URL instead"
	bridgeMetadataAsHashHint = "BridgeMetaDataAsHash is deprecated, remove it from configuration " +
		"(bridge metadata is always stored as hash)"
	aggsenderAgglayerClientHint           = "Use AggSender.AgglayerClient instead"
	aggsenderAggkitProverClientHint       = "Use AggSender.AggkitProverClient instead"
	aggsenderAgglayerClientUseTLSHint     = "Use AggSender.AgglayerClient.UseTLS instead"
	aggsenderAggkitProverClientUseTLSHint = "Use AggSender.AggkitProverClient.UseTLS instead"
	aggsenderUseRequestTimeoutHint        = "Use AggSender.AggkitProverClient.RequestTimeout instead"
	aggchainProofGenUseRequestTimeoutHint = "Use AggchainProofGen.AggkitProverClient.RequestTimeout instead"
	translatorDeprecatedHint              = "Translator parameter is deprecated, remove it from configuration"
	isValidiumModeDeprecatedHint          = "IsValidiumMode parameter is deprecated, remove it from configuration"
	contractVersionsDeprecatedHint        = "ContractVersions parameter is deprecated, remove it from configuration"
	ethermanDeprecatedHint                = "Etherman config is deprecated, remove it from configuration"
	networkConfigDeprecatedHint           = "NetworkConfig is deprecated, use L1NetworkConfig instead"
	l1NetworkConfigUsePolTokenAddrHint    = "Use L1NetworkConfig.POLTokenAddr instead"
	l1NetworkConfigUseRollupAddrHint      = "Use L1NetworkConfig.RollupAddr instead"
	delayBetweenRetriesHint               = "AggSender.DelayBeetweenRetries is deprecated, " +
		"use AggSender.DelayBetweenRetries instead"
)

type DeprecatedFieldsError struct {
	// key is the rule and the value is the field's name that matches the rule
	Fields map[DeprecatedField][]string
}

func NewErrDeprecatedFields() *DeprecatedFieldsError {
	return &DeprecatedFieldsError{
		Fields: make(map[DeprecatedField][]string),
	}
}

func (e *DeprecatedFieldsError) AddDeprecatedField(fieldName string, rule DeprecatedField) {
	e.Fields[rule] = append(e.Fields[rule], fieldName)
}

func (e *DeprecatedFieldsError) Error() string {
	res := "found deprecated fields:"
	for rule, matchingFields := range e.Fields {
		res += fmt.Sprintf("\n\t- %s: %s", strings.Join(matchingFields, ", "), rule.Reason)
	}
	return res
}

type DeprecatedField struct {
	// If the field name ends with a dot means that match a section
	FieldNamePattern string
	Reason           string
}

var (
	deprecatedFieldsOnConfig = []DeprecatedField{
		{
			FieldNamePattern: "L1Config.polygonBridgeAddr",
			Reason:           bridgeAddrSetOnWrongSection,
		},
		{
			FieldNamePattern: "L2Config.polygonBridgeAddr",
			Reason:           bridgeAddrSetOnWrongSection,
		},
		{
			FieldNamePattern: "AggOracle.EVMSender.URLRPCL2",
			Reason:           l2URLHint,
		},
		{
			FieldNamePattern: "AggSender.URLRPCL2",
			Reason:           l2URLHint,
		},
		{
			FieldNamePattern: "AggSender.BridgeMetadataAsHash",
			Reason:           bridgeMetadataAsHashHint,
		},
		{
			FieldNamePattern: "AggSender.AggLayerURL",
			Reason:           aggsenderAgglayerClientHint,
		},
		{
			FieldNamePattern: "AggSender.AggchainProofURL",
			Reason:           aggsenderAggkitProverClientHint,
		},
		{
			FieldNamePattern: "AggchainProofGen.AggchainProofURL",
			Reason:           aggsenderAggkitProverClientHint,
		},
		{
			FieldNamePattern: "AggSender.UseAgglayerTLS",
			Reason:           aggsenderAgglayerClientUseTLSHint,
		},
		{
			FieldNamePattern: "AggSender.UseAggkitProverTLS",
			Reason:           aggsenderAggkitProverClientUseTLSHint,
		},
		{
			FieldNamePattern: "AggSender.GenerateAggchainProofTimeout",
			Reason:           aggsenderUseRequestTimeoutHint,
		},
		{
			FieldNamePattern: "AggchainProofGen.GenerateAggchainProofTimeout",
			Reason:           aggchainProofGenUseRequestTimeoutHint,
		},
		{
			FieldNamePattern: "Common.IsValidiumMode",
			Reason:           isValidiumModeDeprecatedHint,
		},
		{
			FieldNamePattern: "Common.ContractVersions",
			Reason:           contractVersionsDeprecatedHint,
		},
		{
			FieldNamePattern: "Common.Translator",
			Reason:           translatorDeprecatedHint,
		},
		{
			FieldNamePattern: "Etherman",
			Reason:           ethermanDeprecatedHint,
		},
		{
			FieldNamePattern: "NetworkConfig.L1.PolAddr",
			Reason:           l1NetworkConfigUsePolTokenAddrHint,
		},
		{
			FieldNamePattern: "NetworkConfig.L1.ZkEVMAddr",
			Reason:           l1NetworkConfigUseRollupAddrHint,
		},
		{
			FieldNamePattern: "NetworkConfig",
			Reason:           networkConfigDeprecatedHint,
		},
		{
			FieldNamePattern: "Aggsender.DelayBeetweenRetries",
			Reason:           delayBetweenRetriesHint,
		},
	}
)

/*
Config represents the configuration of the entire Aggkit Node
The file is [TOML format]

[TOML format]: https://en.wikipedia.org/wiki/TOML
*/
type Config struct {
	// Configure Log level for all the services, allow also to store the logs in a file
	Log log.Config

	// Common Config that affects all the services
	Common common.Config

	// L1NetworkConfig represents the L1 network config and contains RPC URL alongside L1 contract addresses.
	L1NetworkConfig L1NetworkConfig

	// REST contains the configuration settings for the REST service in the Aggkit
	REST common.RESTConfig

	// RPC is the config for the RPC server
	RPC jRPC.Config

	// Configuration of the reorg detector service to be used for the L1
	ReorgDetectorL1 reorgdetector.Config

	// Configuration of the reorg detector service to be used for the L2
	ReorgDetectorL2 reorgdetector.Config

	// Configuration of the aggOracle service
	AggOracle aggoracle.Config

	// Configuration of the L1 Info Treee Sync service
	L1InfoTreeSync l1infotreesync.Config

	// BridgeL1Sync is the configuration for the synchronizer of the bridge of the L1
	BridgeL1Sync bridgesync.Config

	// BridgeL2Sync is the configuration for the synchronizer of the bridge of the L2
	BridgeL2Sync bridgesync.Config

	// LastGERSync is the config for the synchronizer in charge of syncing the last GER injected on L2.
	// Needed for the bridge service (RPC)
	LastGERSync lastgersync.Config

	// AggSender is the configuration of the agg sender service
	AggSender aggsendercfg.Config

	// Prometheus is the configuration of the prometheus service
	Prometheus prometheus.Config

	// AggchainProofGen is the configuration of the Aggchain Proof Generation Tool
	AggchainProofGen prover.Config

	// Profiling is the configuration of the profiling service
	Profiling pprof.Config
}

// Load loads the configuration
func Load(ctx *cli.Context) (*Config, error) {
	configFilePath := ctx.StringSlice(FlagCfg)
	filesData, err := readFiles(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("error reading files:  Err:%w", err)
	}
	saveConfigPath := ctx.String(FlagSaveConfigPath)
	defaultConfigVars := !ctx.Bool(FlagDisableDefaultConfigVars)
	allowDeprecatedFields := ctx.Bool(FlagAllowDeprecatedFields)
	return LoadFile(filesData, saveConfigPath, defaultConfigVars, allowDeprecatedFields)
}

func readFiles(files []string) ([]FileData, error) {
	result := make([]FileData, 0, len(files))
	for _, file := range files {
		fileContent, err := readFileToString(file)
		if err != nil {
			return nil, fmt.Errorf("error reading file content: %s. Err:%w", file, err)
		}
		fileExtension := getFileExtension(file)
		if fileExtension != ConfigType {
			fileContent, err = convertFileToToml(fileContent, fileExtension)
			if err != nil {
				return nil, fmt.Errorf("error converting file: %s from %s to TOML. Err:%w", file, fileExtension, err)
			}
		}
		result = append(result, FileData{Name: file, Content: fileContent})
	}
	return result, nil
}

func getFileExtension(fileName string) string {
	return fileName[strings.LastIndex(fileName, ".")+1:]
}

// Load loads the configuration
func LoadFileFromString(configFileData string, configType string) (*Config, error) {
	cfg := &Config{}
	err := loadString(cfg, configFileData, configType, true, EnvVarPrefix)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

func SaveConfigToFile(cfg *Config, saveConfigPath string) error {
	marshaled, err := toml.Marshal(cfg)
	if err != nil {
		log.Errorf("Can't marshal config to toml. Err: %w", err)
		return err
	}
	return SaveDataToFile(saveConfigPath, "final config file", marshaled)
}

func SaveDataToFile(fullPath, reason string, data []byte) error {
	log.Infof("Writing %s to: %s", reason, fullPath)
	err := os.WriteFile(fullPath, data, DefaultCreationFilePermissions)
	if err != nil {
		err = fmt.Errorf("error writing %s to file %s. Err: %w", reason, fullPath, err)
		log.Error(err)
		return err
	}
	return nil
}

// Load loads the configuration
func LoadFile(files []FileData, saveConfigPath string,
	setDefaultVars bool, allowDeprecatedFields bool) (*Config, error) {
	log.Infof("Loading configuration: saveConfigPath: %s, setDefaultVars: %t, allowDeprecatedFields: %t",
		saveConfigPath, setDefaultVars, allowDeprecatedFields)
	fileData := make([]FileData, 0)
	if setDefaultVars {
		log.Info("Setting default vars")
		fileData = append(fileData, FileData{Name: "default_mandatory_vars", Content: DefaultMandatoryVars})
	}
	fileData = append(fileData, FileData{Name: "default_vars", Content: DefaultVars})
	fileData = append(fileData, FileData{Name: "default_values", Content: DefaultValues})
	fileData = append(fileData, files...)

	merger := NewConfigRender(fileData, EnvVarPrefix)

	renderedCfg, err := merger.Render()
	if err != nil {
		return nil, err
	}
	if saveConfigPath != "" {
		fullPath := filepath.Join(saveConfigPath, fmt.Sprintf("%s.merged", SaveConfigFileName))
		err = SaveDataToFile(fullPath, "merged config file", []byte(renderedCfg))
		if err != nil {
			return nil, err
		}
	}
	cfg, err := LoadFileFromString(renderedCfg, ConfigType)
	// If allowDeprecatedFields is true, we ignore the deprecated fields
	if err != nil && allowDeprecatedFields {
		var customErr *DeprecatedFieldsError
		if errors.As(err, &customErr) {
			log.Warnf("detected deprecated fields: %s", err.Error())
			err = nil
		}
	}

	if err != nil {
		return nil, err
	}
	if saveConfigPath != "" {
		fullPath := saveConfigPath + "/" + SaveConfigFileName
		err = SaveConfigToFile(cfg, fullPath)
		if err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// Load loads the configuration
func loadString(cfg *Config, configData string, configType string,
	allowEnvVars bool, envPrefix string) error {
	viper.SetConfigType(configType)
	if allowEnvVars {
		replacer := strings.NewReplacer(".", "_")
		viper.SetEnvKeyReplacer(replacer)
		viper.SetEnvPrefix(envPrefix)
		viper.AutomaticEnv()
	}
	err := viper.ReadConfig(bytes.NewBuffer([]byte(configData)))
	if err != nil {
		return err
	}
	decodeHooks := []viper.DecoderConfigOption{
		// this allows arrays to be decoded from env var separated by ",", example: MY_VAR="value1,value2,value3"
		viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
			mapstructure.TextUnmarshallerHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		)),
	}

	err = viper.Unmarshal(&cfg, decodeHooks...)
	if err != nil {
		return err
	}
	configKeys := viper.AllKeys()
	err = checkDeprecatedFields(configKeys)
	if err != nil {
		return err
	}

	return nil
}

func checkDeprecatedFields(keysOnConfig []string) error {
	err := NewErrDeprecatedFields()
	for _, key := range keysOnConfig {
		forbbidenInfo := getDeprecatedField(key)
		if forbbidenInfo != nil {
			err.AddDeprecatedField(key, *forbbidenInfo)
		}
	}
	if len(err.Fields) > 0 {
		return err
	}
	return nil
}

func getDeprecatedField(fieldName string) *DeprecatedField {
	field := strings.ToLower(fieldName)
	for _, deprecatedField := range deprecatedFieldsOnConfig {
		pattern := strings.ToLower(deprecatedField.FieldNamePattern)

		// Exact match
		if pattern == field {
			return &deprecatedField
		}

		// Prefix match if the pattern represents a section
		if strings.HasSuffix(pattern, ".") {
			if strings.HasPrefix(field, pattern) {
				return &deprecatedField
			}
		} else {
			// If it's a section, match everything under it
			if strings.HasPrefix(field, pattern+".") {
				return &deprecatedField
			}
		}
	}
	return nil
}
