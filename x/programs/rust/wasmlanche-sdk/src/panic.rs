pub fn register_panic() {
    std::panic::set_hook(Box::new(|info| {
        let payload = info.payload();
        let payload = if let Some(s) = payload.downcast_ref::<&str>() {
            s
        } else if let Some(s) = payload.downcast_ref::<String>() {
            s
        } else {
            "uncaught payload"
        };

        let (file, line, column) = info
            .location()
            .map(|loc| (loc.file(), loc.line().to_string(), loc.column().to_string()))
            .unwrap_or(("???", "???".into(), "???".into()));

        let as_string = format!("[{}:{}:{}] {:#?}", file, line, column, payload);
        crate::log(as_string.as_str());
    }));
}
