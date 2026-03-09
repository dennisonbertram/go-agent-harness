Cancel a pending delayed callback by its ID. Use this to stop a previously scheduled one-shot callback before it fires.

Returns an error if the callback has already fired or was already canceled. Use list_delayed_callbacks first if you need to find the callback ID.