package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gustavoohrodrigues/permguard/internal/audit"
	"github.com/gustavoohrodrigues/permguard/internal/change"
	"github.com/gustavoohrodrigues/permguard/internal/config"
	"github.com/gustavoohrodrigues/permguard/internal/domain"
	"github.com/gustavoohrodrigues/permguard/internal/filesystem"
	"github.com/gustavoohrodrigues/permguard/internal/i18n"
	"github.com/gustavoohrodrigues/permguard/internal/permissions"
	"github.com/gustavoohrodrigues/permguard/internal/privilege"
	"github.com/gustavoohrodrigues/permguard/internal/tui"
)

type Application struct {
	lang, configPath string
	allowWrites      bool
}

func New() *Application { return &Application{} }

func (a *Application) Execute() error {
	preLang := os.Getenv("PERMGUARD_LANG")
	if preLang == "" {
		preLang = "pt-BR"
	}
	preCatalog, _ := i18n.Load(preLang)
	root := &cobra.Command{Use: "permguard", Short: preCatalog.T("cli.short"), SilenceUsage: true, SilenceErrors: true}
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().StringVar(&a.lang, "lang", "", preCatalog.T("cli.lang"))
	root.PersistentFlags().StringVar(&a.configPath, "config", "", preCatalog.T("cli.config"))
	root.PersistentFlags().BoolVar(&a.allowWrites, "permitir-alteracoes", false, preCatalog.T("cli.allow_writes"))
	root.RunE = func(cmd *cobra.Command, args []string) error {
		cfg, catalog, err := a.runtime()
		if err != nil {
			return err
		}
		privileges := privilege.Detect()
		auditPath, err := audit.Path(cfg.Audit.UserPath, cfg.Audit.RootPath, privileges.IsRoot)
		if err != nil {
			return err
		}
		inspector := filesystem.Inspector{}
		var auditWriter tui.AuditWriter
		if cfg.Audit.Enabled {
			auditWriter = audit.Writer{Path: auditPath}
		}
		return tui.Run(tui.Dependencies{Catalog: catalog, Inspector: inspector, Privilege: privileges, Config: cfg, Changer: change.Service{Inspector: inspector}, Audit: auditWriter, AllowWrites: a.allowWrites})
	}
	root.AddCommand(a.inspectCommand(preCatalog), a.permissionCommand(preCatalog), a.changePermissionCommand(preCatalog))
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) { printHelp(cmd, preCatalog) })
	if err := root.Execute(); err != nil {
		return fmt.Errorf(preCatalog.T("error.prefix"), localizedError(preCatalog, err))
	}
	return nil
}

func (a *Application) changePermissionCommand(initial *i18n.Catalog) *cobra.Command {
	var modeValue, confirmation string
	command := &cobra.Command{Use: "alterar-permissao CAMINHO", Short: initial.T("cli.change_permission.short"), Args: exactOne, RunE: func(cmd *cobra.Command, args []string) error {
		if !a.allowWrites {
			return fmt.Errorf("escrita_nao_habilitada")
		}
		cfg, catalog, err := a.runtime()
		if err != nil {
			return err
		}
		mode, err := permissions.Parse(modeValue)
		if err != nil {
			return err
		}
		inspector := filesystem.Inspector{}
		metadata, err := inspector.Inspect(args[0])
		if err != nil {
			return err
		}
		privileges := privilege.Detect()
		service := change.Service{Inspector: inspector}
		if err := service.Validate(metadata, privileges.EffectiveUID); err != nil {
			return err
		}
		proposed := permissions.FromFileMode(mode)
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s: %s\n%s: %s · %s\n%s: %s · %s\n", catalog.T("change.preview_title"), catalog.T("label.current_path"), metadata.Path, catalog.T("change.current"), metadata.Mode.NumericMode, metadata.Mode.SymbolicMode, catalog.T("change.proposed"), proposed.NumericMode, proposed.SymbolicMode); err != nil {
			return err
		}
		if confirmation == "" {
			if _, err := fmt.Fprint(cmd.OutOrStdout(), catalog.T("label.confirmation_input")+": "); err != nil {
				return err
			}
			scanner := bufio.NewScanner(cmd.InOrStdin())
			if !scanner.Scan() {
				return fmt.Errorf("confirmacao_invalida")
			}
			confirmation = strings.TrimSpace(scanner.Text())
		}
		if confirmation != cfg.Security.ConfirmationWord {
			return fmt.Errorf("confirmacao_invalida")
		}
		applyErr := service.ApplyMode(metadata, mode, privileges.EffectiveUID)
		result, errorText := "sucesso", ""
		if applyErr != nil {
			result, errorText = "falha", applyErr.Error()
		}
		if cfg.Audit.Enabled {
			path, pathErr := audit.Path(cfg.Audit.UserPath, cfg.Audit.RootPath, privileges.IsRoot)
			if pathErr != nil {
				return pathErr
			}
			record := domain.AuditRecord{Timestamp: time.Now(), OperatorUser: privileges.User, OperatorUID: privileges.EffectiveUID, TargetPath: metadata.Path, TargetType: metadata.Type, Operation: "chmod", PreviousMode: metadata.Mode.NumericMode, NewMode: proposed.NumericMode, PreviousOwner: metadata.Owner.Name, PreviousGroup: metadata.Group.Name, Result: result, Error: errorText}
			if auditErr := (audit.Writer{Path: path}).Append(record); auditErr != nil && applyErr == nil {
				return auditErr
			}
		}
		if applyErr != nil {
			return applyErr
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), catalog.T("status.permission_changed"))
		return err
	}}
	command.Flags().StringVar(&modeValue, "modo", "", initial.T("cli.mode"))
	command.Flags().StringVar(&confirmation, "confirmar", "", initial.T("cli.confirm"))
	_ = command.MarkFlagRequired("modo")
	return command
}

func (a *Application) runtime() (config.Config, *i18n.Catalog, error) {
	cfg, err := config.Load(a.configPath)
	if err != nil {
		return cfg, nil, err
	}
	language := a.lang
	if language == "" {
		language = os.Getenv("PERMGUARD_LANG")
	}
	if language == "" {
		language = cfg.Language
	}
	catalog, err := i18n.Load(language)
	return cfg, catalog, err
}

func (a *Application) inspectCommand(initial *i18n.Catalog) *cobra.Command {
	return &cobra.Command{Use: "inspecionar CAMINHO", Short: initial.T("cli.inspect.short"), Args: exactOne, RunE: func(cmd *cobra.Command, args []string) error {
		_, catalog, err := a.runtime()
		if err != nil {
			return err
		}
		metadata, err := (filesystem.Inspector{}).Inspect(args[0])
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n%s: %s\n%s: %s (%s)\n%s: %s (%s)\n%s: %s · %s\n%s: %d bytes\n%s: %d\n",
			catalog.T("label.current_path"), metadata.Path, catalog.T("label.type"), catalog.T("type."+string(metadata.Type)),
			catalog.T("label.owner"), metadata.Owner.Name, metadata.Owner.UID, catalog.T("label.group"), metadata.Group.Name, metadata.Group.GID,
			catalog.T("label.permissions"), metadata.Mode.NumericMode, metadata.Mode.SymbolicMode, catalog.T("label.size"), metadata.Size, catalog.T("label.inode"), metadata.Inode); err != nil {
			return err
		}
		if metadata.IsSymlink {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s · %s: %s\n", catalog.T("label.symlink"), catalog.T("label.yes"), catalog.T("label.target"), metadata.SymlinkTarget); err != nil {
				return err
			}
		}
		return nil
	}}
}

func (a *Application) permissionCommand(initial *i18n.Catalog) *cobra.Command {
	permission := &cobra.Command{Use: "permissao", Short: initial.T("cli.permission.short")}
	explain := &cobra.Command{Use: "explicar MODO", Short: initial.T("cli.explain.short"), Args: exactOne, RunE: func(cmd *cobra.Command, args []string) error {
		_, catalog, err := a.runtime()
		if err != nil {
			return err
		}
		mode, err := permissions.Parse(args[0])
		if err != nil {
			return err
		}
		info := permissions.FromFileMode(mode)
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n%s: %s\n\n", catalog.T("label.numeric"), info.NumericMode, catalog.T("label.symbolic"), info.SymbolicMode); err != nil {
			return err
		}
		for _, scope := range permissions.Explain(info, false) {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s (%s):\n", catalog.T("scope."+scope.Scope), scope.Bits); err != nil {
				return err
			}
			for _, code := range scope.Codes {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "- %s.\n", catalog.T(code)); err != nil {
					return err
				}
			}
		}
		if info.SUID {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), catalog.T("special.suid")); err != nil {
				return err
			}
		}
		if info.SGID {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), catalog.T("special.sgid.file")); err != nil {
				return err
			}
		}
		if info.Sticky {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), catalog.T("special.sticky")); err != nil {
				return err
			}
		}
		return nil
	}}
	permission.AddCommand(explain)
	return permission
}

func localizedError(c *i18n.Catalog, err error) string {
	code := err.Error()
	base, _, _ := strings.Cut(code, ": ")
	translated := c.T("error." + base)
	if strings.HasPrefix(translated, "[") {
		return code
	}
	return translated
}

func exactOne(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("quantidade_argumentos") //nolint:misspell // identificador interno em pt-BR.
	}
	return nil
}

func printHelp(cmd *cobra.Command, catalog *i18n.Catalog) {
	out := cmd.OutOrStdout()
	useLine := strings.ReplaceAll(cmd.UseLine(), "[flags]", "[opções]")
	_, _ = fmt.Fprintf(out, "%s:\n  %s\n\n%s\n\n", catalog.T("cli.usage"), useLine, cmd.Short)
	if commands := cmd.Commands(); len(commands) > 0 {
		_, _ = fmt.Fprintln(out, catalog.T("cli.commands")+":")
		for _, child := range commands {
			if child.IsAvailableCommand() {
				_, _ = fmt.Fprintf(out, "  %-18s %s\n", child.Name(), child.Short)
			}
		}
		_, _ = fmt.Fprintln(out)
	}
	if cmd.HasAvailableLocalFlags() || cmd.HasAvailableInheritedFlags() {
		_, _ = fmt.Fprintln(out, catalog.T("cli.flags")+":")
		seen := make(map[string]bool)
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			seen[flag.Name] = true
			usage := flag.Usage
			if flag.Name == "help" {
				usage = catalog.T("cli.help")
			}
			_, _ = fmt.Fprintf(out, "  --%-16s %s\n", flag.Name, usage)
		})
		cmd.InheritedFlags().VisitAll(func(flag *pflag.Flag) {
			if !seen[flag.Name] {
				_, _ = fmt.Fprintf(out, "  --%-16s %s\n", flag.Name, flag.Usage)
			}
		})
		_, _ = fmt.Fprintln(out)
	}
	if cmd.HasAvailableSubCommands() {
		_, _ = fmt.Fprintf(out, catalog.T("cli.more")+"\n", cmd.CommandPath())
	}
}

func ResolveStartPath(value string) string {
	if value == "" {
		value = "."
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return value
	}
	return absolute
}
