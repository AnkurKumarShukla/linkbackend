// SPDX-License-Identifier: MIT
pragma solidity 0.8.19;

import {FunctionsClient} from "@chainlink/contracts@1.4.0/src/v0.8/functions/v1_0_0/FunctionsClient.sol";
import {ConfirmedOwner} from "@chainlink/contracts@1.4.0/src/v0.8/shared/access/ConfirmedOwner.sol";
import {FunctionsRequest} from "@chainlink/contracts@1.4.0/src/v0.8/functions/v1_0_0/libraries/FunctionsRequest.sol";

/**
 * @title GettingStartedFunctionsConsumer
 * @notice Example contract to fetch and store Reflexivity Score using Chainlink Functions
 */
contract GettingStartedFunctionsConsumer is FunctionsClient, ConfirmedOwner {
    using FunctionsRequest for FunctionsRequest.Request;

    // State variables
    bytes32 public s_lastRequestId;
    bytes public s_lastResponse;
    bytes public s_lastError;
    uint256 public Reflexivity_Score;

    // Custom error
    error UnexpectedRequestID(bytes32 requestId);

    // Event to log responses
    event Response(
        bytes32 indexed requestId,
        uint256 reflexivityScore,
        bytes response,
        bytes err
    );

    // Chainlink Functions Router address (Sepolia)
    address router = 0xb83E47C2bC239B3bf370bc41e1459A34b41238D0;

    // Chainlink DON ID (Sepolia)
    bytes32 donID =
        0x66756e2d657468657265756d2d7365706f6c69612d3100000000000000000000;

    // JavaScript source code executed by Chainlink Functions
    string source = "const characterId = args[0]; let stream_id = \"\"; if (characterId === \"ba782\") { stream_id = \"0x000359843a543ee2fe414dc14c7e7920ef10f4372990b79d6361cdc0dd1ba782\"; } else if (characterId === \"5b439\") { stream_id = \"0x00037da06d56d083fe599397a4769a042d63aa73dc4ef57709d31e9971a5b439\"; } else if (characterId === \"754d6\") { stream_id = \"0x0003d338ea2ac3be9e026033b1aa601673c37bab5e13851c59966f9f820754d6\"; } else { throw Error(\"Invalid characterId provided.\"); } const apiResponse = await Functions.makeHttpRequest({ url: `http://13.234.66.243:8000/reflexivity?stream_id=${stream_id}` }); if (apiResponse.error) { throw Error('Request failed'); } const { data } = apiResponse; if (typeof data.reflexivity_score !== 'number') { throw Error('Invalid reflexivity_score received from API'); } const reflexivityScaled = Math.round(data.reflexivity_score * 1_000_000); return Functions.encodeUint256(reflexivityScaled);";

    // Callback gas limit
    uint32 gasLimit = 300000;

    /**
     * @notice Initializes the contract
     */
    constructor() FunctionsClient(router) ConfirmedOwner(msg.sender) {}

    /**
     * @notice Sends a Chainlink Functions request to fetch Reflexivity Score
     * @param subscriptionId The Chainlink subscription ID
     * @param args Arguments to pass (expects characterId)
     * @return requestId The unique request ID
     */
    function sendRequest(
        uint64 subscriptionId,
        string[] calldata args
    ) external onlyOwner returns (bytes32 requestId) {
        FunctionsRequest.Request memory req;
        req.initializeRequestForInlineJavaScript(source);

        if (args.length > 0) {
            req.setArgs(args);
        }

        s_lastRequestId = _sendRequest(
            req.encodeCBOR(),
            subscriptionId,
            gasLimit,
            donID
        );

        return s_lastRequestId;
    }

    /**
     * @notice Callback function to fulfill Chainlink Functions request
     * @param requestId Unique request ID
     * @param response Encoded response (uint256 encoded as bytes)
     * @param err Any errors returned
     */
    function fulfillRequest(
        bytes32 requestId,
        bytes memory response,
        bytes memory err
    ) internal override {
        if (s_lastRequestId != requestId) {
            revert UnexpectedRequestID(requestId);
        }

        s_lastResponse = response;
        s_lastError = err;

        uint256 decodedScore = abi.decode(response, (uint256));
        Reflexivity_Score = decodedScore;

        emit Response(requestId, decodedScore, s_lastResponse, s_lastError);
    }
    function getReflexivityScore() external view returns(uint256) {
        return Reflexivity_Score;
}
}
