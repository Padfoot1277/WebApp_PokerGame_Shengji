// ===== Server -> Client =====

export type HelloMsg = {
    type: 'hello'
    uid: string
}

export type SnapshotMsg<T = any> = {
    type: 'snapshot'
    state: T
}

export type ErrorMsg = {
    type: 'error'
    message: string
}

export type NoticeMsg = {
    type: 'notice'
    message: string
}

export type ServerMessage =
    | HelloMsg
    | SnapshotMsg
    | ErrorMsg
    | NoticeMsg

// ===== Client -> Server =====

export type ClientEvent<T = any> = {
    type: string
    payload: T
}
