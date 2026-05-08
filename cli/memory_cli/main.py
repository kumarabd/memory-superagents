import typer

app = typer.Typer(
    name="memory",
    help="Claude Memory — operational control plane for your memory platform.",
    no_args_is_help=True,
)


@app.callback(invoke_without_command=True)
def main(ctx: typer.Context) -> None:
    """Claude Memory — operational control plane for your memory platform."""
    if ctx.invoked_subcommand is None:
        typer.echo(ctx.get_help())


if __name__ == "__main__":
    app()
