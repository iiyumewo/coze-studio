def format_report(title, items):
    lines = [f"# {title}", ""]
    lines.extend(f"{index}. {item}" for index, item in enumerate(items, start=1))
    return "\n".join(lines) + "\n"
