/* eslint-disable */
/* tslint:disable */
// @ts-nocheck
/*
 * ---------------------------------------------------------------
 * ## THIS FILE WAS GENERATED VIA SWAGGER-TYPESCRIPT-API        ##
 * ##                                                           ##
 * ## AUTHOR: acacode                                           ##
 * ## SOURCE: https://github.com/acacode/swagger-typescript-api ##
 * ---------------------------------------------------------------
 */

import httpRequest, { ContentType, RequestParams } from "./httpClient";
import {
  DomainPWResponse,
  DomainPrompt,
  DomainUpdatePromptReq,
  GetApiProV1PromptParams,
} from "./types";

/**
 * @description Get all prompts
 *
 * @tags prompt
 * @name GetApiProV1Prompt
 * @summary Get all prompts
 * @request GET:/api/pro/v1/prompt
 * @response `200` `(DomainPWResponse & {
    data?: DomainPrompt,

})` OK
 */

export const getApiProV1Prompt = (
  query: GetApiProV1PromptParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainPWResponse & {
      data?: DomainPrompt;
    }
  >({
    path: `/api/pro/v1/prompt`,
    method: "GET",
    query: query,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description update prompt settings
 *
 * @tags prompt
 * @name PutApiProV1Prompt
 * @summary update prompt settings
 * @request PUT:/api/pro/v1/prompt
 * @response `200` `(DomainPWResponse & {
    data?: DomainPrompt,

})` OK
 */

export const putApiProV1Prompt = (
  req: DomainUpdatePromptReq,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainPWResponse & {
      data?: DomainPrompt;
    }
  >({
    path: `/api/pro/v1/prompt`,
    method: "PUT",
    body: req,
    type: ContentType.Json,
    format: "json",
    ...params,
  });
