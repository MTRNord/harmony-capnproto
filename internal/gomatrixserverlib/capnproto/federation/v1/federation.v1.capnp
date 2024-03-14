using Go = import "/go.capnp";
using Types = import "./types/types.capnp";

@0xee8fadeb6a9300eb;

$Go.package("v1");
$Go.import("github.com/neilalexander/harmony/internal/gomatrixserverlib/capnproto/federation/v1");

annotation methodUUID @0xe55590d1a37e8043 (method) :UInt64;

# This is an async stream callback. This is a callback called on the other end
interface StreamCallback(T) {
    # Data is written here. It behaves like a regular callback on the receiver and a function call on the caller
    write @0 (value :T) -> stream;
    # The stream will be closed when this is called.
    done @1 ();
}

interface MatrixFederation @0xf730448b3b47991e {
    # Get the implementation name and version of this homeserver.
    getVersion @0 () -> (serverVersion :Types.ServerVersion) $methodUUID(0xab1eb3e81f3344d1);

    # This combines the query via other servers and the direct query endpoints
    getKeys @1 (server_keys :Types.Map(Text, Types.Map(Text, Types.QueryCriteria)), callback :StreamCallback(Types.ServerKeysResponse)) -> () $methodUUID(0x932dd11596dad50e);

    # Push messages representing live activity to another server. The destination
    # name will be set to that of the receiving server itself. Each embedded PDU
    # in the transaction body will be processed.
    #
    # Errors are returned as stream errors rather than at the end of the stream.
    sendTransactions @2 () -> (callback :StreamCallback(Types.Transaction)) $methodUUID(0xec6b6ce167005c84);

    # [https://spec.matrix.org/v1.9/server-server-api/#get_matrixfederationv1backfillroomid](https://spec.matrix.org/v1.9/server-server-api/#get_matrixfederationv1backfillroomid)
    # Retrieves a sliding-window history of previous PDUs that occurred in the
    # given room. Starting from the PDU ID(s) given in the v argument, the PDUs
    # given in v and the PDUs that preceded them are retrieved, up to the total
    # number given by the limit.
    backfill @3 (auth_data :Types.AuthData, roomID :Text, limit :UInt32, eventIDs :List(Text), callback :StreamCallback(Types.BackfillData)) -> () $methodUUID(0x970e8e8dfe8f0ced);
}