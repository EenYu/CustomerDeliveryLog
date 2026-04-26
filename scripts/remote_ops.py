#!/usr/bin/env python3

import argparse
import getpass
import os
import re
import shlex
import sys
from pathlib import Path

import paramiko


DEFAULT_HOST = os.environ.get("REMOTE_HOST", "192.168.203.131")
DEFAULT_PORT = int(os.environ.get("REMOTE_PORT", "22"))
DEFAULT_USER = os.environ.get("REMOTE_USER", "root")
DEFAULT_APP_DIR = os.environ.get("REMOTE_APP_DIR", "/home/gxp/customer-delivery-log")


def default_run_as(app_dir: str, ssh_user: str) -> str:
    env_value = os.environ.get("REMOTE_RUN_AS")
    if env_value:
        return env_value
    match = re.match(r"^/home/([^/]+)/", app_dir)
    if match:
        return match.group(1)
    return ssh_user


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Remote operations for customer-delivery-log")
    parser.add_argument(
        "action",
        choices=[
            "sync-scripts",
            "start",
            "stop",
            "restart",
            "status",
            "health",
            "logs",
            "nginx-test",
            "nginx-reload",
        ],
        help="operation to execute on the remote host",
    )
    parser.add_argument("--host", default=DEFAULT_HOST, help="remote host")
    parser.add_argument("--port", type=int, default=DEFAULT_PORT, help="remote ssh port")
    parser.add_argument("--user", default=DEFAULT_USER, help="remote ssh user")
    parser.add_argument("--password", default=os.environ.get("REMOTE_PASSWORD"), help="remote ssh password")
    parser.add_argument("--app-dir", default=DEFAULT_APP_DIR, help="remote application directory")
    parser.add_argument("--run-as", help="remote application user for app start/stop/restart")
    parser.add_argument(
        "--root-dir",
        default=str(Path(__file__).resolve().parents[1]),
        help="local project root used by sync-scripts",
    )
    return parser


def exec_remote(client: paramiko.SSHClient, command: str) -> int:
    wrapped = "bash -lc " + shlex.quote(command)
    stdin, stdout, stderr = client.exec_command(wrapped, get_pty=True)
    out = stdout.read().decode("utf-8", "ignore")
    err = stderr.read().decode("utf-8", "ignore")
    if out:
        print(out, end="" if out.endswith("\n") else "\n")
    if err:
        print(err, end="" if err.endswith("\n") else "\n", file=sys.stderr)
    return stdout.channel.recv_exit_status()


def wrap_run_as(command: str, ssh_user: str, run_as: str) -> str:
    if not run_as or run_as == ssh_user:
        return command
    return f"su -s /bin/bash - {shlex.quote(run_as)} -c {shlex.quote(command)}"


def sync_scripts(client: paramiko.SSHClient, root_dir: Path, app_dir: str) -> int:
    sftp = client.open_sftp()
    local_dir = root_dir / "deploy" / "linux"
    remote_files = {
        "start.sh": f"{app_dir}/start.sh",
        "stop.sh": f"{app_dir}/stop.sh",
        "restart.sh": f"{app_dir}/restart.sh",
    }

    try:
        for name, remote_path in remote_files.items():
            local_path = local_dir / name
            if not local_path.is_file():
                raise FileNotFoundError(f"local file not found: {local_path}")
            sftp.put(str(local_path), remote_path)
            print(f"uploaded: {local_path} -> {remote_path}")
    finally:
        sftp.close()

    chmod_command = (
        f"chmod 755 {shlex.quote(app_dir)}/start.sh "
        f"{shlex.quote(app_dir)}/stop.sh "
        f"{shlex.quote(app_dir)}/restart.sh "
        f"{shlex.quote(app_dir)}/bin/customer-delivery-log"
    )
    return exec_remote(client, chmod_command)


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    run_as = args.run_as or default_run_as(args.app_dir, args.user)

    password = args.password or getpass.getpass(f"Password for {args.user}@{args.host}: ")

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())

    try:
        client.connect(
            args.host,
            port=args.port,
            username=args.user,
            password=password,
            timeout=20,
        )

        app_dir = args.app_dir.rstrip("/")
        commands = {
            "start": wrap_run_as(f"cd {shlex.quote(app_dir)} && ./start.sh", args.user, run_as),
            "stop": wrap_run_as(f"cd {shlex.quote(app_dir)} && ./stop.sh", args.user, run_as),
            "restart": wrap_run_as(f"cd {shlex.quote(app_dir)} && ./restart.sh", args.user, run_as),
            "health": (
                "echo '== app ==' && "
                "curl -fsS http://127.0.0.1:8080/api/v1/health && echo && "
                "echo '== nginx ==' && "
                "curl -fsS http://127.0.0.1/api/v1/health && echo"
            ),
            "logs": wrap_run_as(f"tail -n 100 {shlex.quote(app_dir)}/logs/app.out", args.user, run_as),
            "nginx-test": "nginx -t",
            "nginx-reload": "nginx -t && nginx -s reload",
            "status": wrap_run_as(
                f"echo '== pid ==' && "
                f"(test -f {shlex.quote(app_dir)}/run/app.pid && cat {shlex.quote(app_dir)}/run/app.pid || echo 'no pid file') && "
                "echo && echo '== process ==' && "
                "ps -ef | grep customer-delivery-log | grep -v grep || true && "
                "echo && echo '== app health ==' && "
                "curl -fsS http://127.0.0.1:8080/api/v1/health || true && "
                "echo && echo '== nginx health ==' && "
                "curl -fsS http://127.0.0.1/api/v1/health || true && echo",
                args.user,
                run_as,
            ),
        }

        if args.action == "sync-scripts":
            return sync_scripts(client, Path(args.root_dir), app_dir)

        return exec_remote(client, commands[args.action])
    finally:
        client.close()


if __name__ == "__main__":
    sys.exit(main())
