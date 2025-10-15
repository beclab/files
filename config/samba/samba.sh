#!/usr/bin/env bash
set -Eeuo pipefail

# Set variables for group and share directory
group="smb"
share="/storage"
secret="/run/secrets/pass"
config="/etc/samba/smb.conf"

# Create shared directory
mkdir -p "$share" || { echo "Failed to create directory $share"; exit 1; }

# Check if the secret file exists and if its size is greater than zero
if [ -s "$secret" ]; then
    PASS=$(cat "$secret")
fi

# Check if config file is not a directory
if [ -d "$config" ]; then

    echo "The bind $config maps to a file that does not exist!"
    exit 1

fi

# Check if an external config file was supplied
if [ -f "$config" ] && [ -s "$config" ]; then

    # Inform the user we are using a custom configuration file.
    echo "Using provided configuration file: $config."
fi

# Create directories if missing
mkdir -p /var/lib/samba/sysvol
mkdir -p /var/lib/samba/private
mkdir -p /var/lib/samba/bind-dns

# Store configuration location for Healthcheck
ln -sf "$config" /etc/samba.conf

# Set directory permissions
[ -d /run/samba/msg.lock ] && chmod -R 0755 /run/samba/msg.lock
[ -d /var/log/samba/cores ] && chmod -R 0700 /var/log/samba/cores
[ -d /var/cache/samba/msg.lock ] && chmod -R 0755 /var/cache/samba/msg.lock

# Start the Samba daemon with the following options:
#  --configfile: Location of the configuration file.
#  --foreground: Run in the foreground instead of daemonizing.
#  --debug-stdout: Send debug output to stdout.
#  --debuglevel=1: Set debug verbosity level to 1.
#  --no-process-group: Don't create a new process group for the daemon.
# exec smbd --configfile="$config" --foreground --debug-stdout --debuglevel=1 --no-process-group
exec smbd --configfile="$config" --debuglevel=1 --no-process-group