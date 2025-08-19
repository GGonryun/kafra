{ config, lib, pkgs, ... }:

with lib;

let
  cfg = config.services.p0-ssh-agent;
  
  # Create a derivation for the p0-ssh-agent binary
  p0-ssh-agent = pkgs.stdenv.mkDerivation {
    pname = "p0-ssh-agent";
    version = "1.0.0";
    
    # The binary should be provided by the installation process
    src = /usr/bin/p0-ssh-agent;
    
    buildPhase = ":"; # No build needed, just copy
    
    installPhase = ''
      mkdir -p $out/bin
      cp $src $out/bin/p0-ssh-agent
      chmod +x $out/bin/p0-ssh-agent
    '';
  };
  
  configFile = pkgs.writeText "p0-ssh-agent-config.yaml" (builtins.toJSON cfg.config);
  
in {
  options.services.p0-ssh-agent = {
    enable = mkEnableOption "P0 SSH Agent - Secure SSH access management";
    
    config = mkOption {
      type = types.attrs;
      default = {};
      description = "Configuration for P0 SSH Agent as Nix attributes";
      example = {
        version = "1.0";
        orgId = "your-org";
        hostId = "your-host-id";
        environment = "production";
        tunnelHost = "wss://your-tunnel.example.com";
        keyPath = "/etc/p0-ssh-agent/keys";
        logPath = "/var/log/p0-ssh-agent/service.log";
        labels = [ "type=production" ];
        heartbeatIntervalSeconds = 60;
      };
    };
    
    configFile = mkOption {
      type = types.path;
      description = "Path to the configuration file";
      default = "/etc/p0-ssh-agent/config.yaml";
    };
    
    binaryPath = mkOption {
      type = types.path;
      description = "Path to the p0-ssh-agent binary";
      default = "/usr/bin/p0-ssh-agent";
    };
  };
  
  config = mkIf cfg.enable {
    # Enable systemd-homed for JIT user management
    services.homed.enable = mkDefault true;
    
    # Create configuration directory
    system.activationScripts.p0-ssh-agent-setup = ''
      mkdir -p /etc/p0-ssh-agent
      mkdir -p /var/log/p0-ssh-agent
      chown root:root /etc/p0-ssh-agent /var/log/p0-ssh-agent
      chmod 755 /etc/p0-ssh-agent /var/log/p0-ssh-agent
    '';
    
    # Generate YAML config file from Nix configuration
    environment.etc."p0-ssh-agent/config.yaml" = mkIf (cfg.config != {}) {
      text = ''
        version: "${cfg.config.version or "1.0"}"
        orgId: "${cfg.config.orgId or ""}"
        hostId: "${cfg.config.hostId or ""}"
        ${lib.optionalString (cfg.config ? hostname) ''hostname: "${cfg.config.hostname}"''}
        environment: "${cfg.config.environment or "production"}"
        tunnelHost: "${cfg.config.tunnelHost or ""}"
        keyPath: "${cfg.config.keyPath or "/etc/p0-ssh-agent/keys"}"
        logPath: "${cfg.config.logPath or "/var/log/p0-ssh-agent/service.log"}"
        ${lib.optionalString (cfg.config ? labels) ''labels: ${builtins.toJSON cfg.config.labels}''}
        heartbeatIntervalSeconds: ${toString (cfg.config.heartbeatIntervalSeconds or 60)}
        ${lib.optionalString (cfg.config ? dryRun) ''dryRun: ${if cfg.config.dryRun then "true" else "false"}''}
      '';
      mode = "0644";
    };
    
    # Main systemd service
    systemd.services.p0-ssh-agent = {
      enable = true;
      description = "P0 SSH Agent - Secure SSH access management";
      documentation = [ "https://docs.p0.com/" ];
      after = [ "network-online.target" "systemd-homed.service" ];
      wants = [ "network-online.target" ];
      wantedBy = [ "multi-user.target" ];
      
      startLimitIntervalSec = 60;
      startLimitBurst = 10;
      
      serviceConfig = {
        Type = "simple";
        User = "root";
        Group = "root";
        WorkingDirectory = "/etc/p0-ssh-agent";
        ExecStart = "${cfg.binaryPath} start --config ${cfg.configFile}";
        ExecReload = "/bin/kill -HUP $MAINPID";
        Restart = "always";
        RestartSec = "5s";
        StandardOutput = "journal";
        StandardError = "journal";
        SyslogIdentifier = "p0-ssh-agent";
        
        # Ensure service runs independently of user sessions
        RemainAfterExit = false;
        KillMode = "mixed";
        
        # Security settings
        ProtectKernelTunables = true;
        ProtectKernelModules = true;
        ProtectControlGroups = true;
      };
      
      # Environment variables - extend PATH to include system binaries needed for user management
      environment = {
        PATH = lib.mkForce "/run/current-system/sw/bin:/run/current-system/sw/sbin:/run/wrappers/bin:/usr/bin:/bin";
        HOME = "/root";
      };
    };
    
    # Add p0-ssh-agent command to system PATH
    environment.systemPackages = mkIf (builtins.pathExists cfg.binaryPath) [
      (pkgs.runCommand "p0-ssh-agent-wrapper" {} ''
        mkdir -p $out/bin
        ln -s ${cfg.binaryPath} $out/bin/p0-ssh-agent
      '')
    ];
  };
}