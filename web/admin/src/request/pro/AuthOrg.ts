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
  DomainResponse,
  GithubComChaitinPandaWikiProApiAuthV1AuthGroupSyncReq,
  GithubComChaitinPandaWikiProApiAuthV1AuthGroupSyncResp,
} from "./types";

/**
 * @description 组织架构同步
 *
 * @tags AuthOrg
 * @name PostApiProV1AuthGroupSync
 * @summary 组织架构同步
 * @request POST:/api/pro/v1/auth/group/sync
 * @secure
 * @response `200` `(DomainResponse & {
    data?: GithubComChaitinPandaWikiProApiAuthV1AuthGroupSyncResp,

})` OK
 */

export const postApiProV1AuthGroupSync = (
  param: GithubComChaitinPandaWikiProApiAuthV1AuthGroupSyncReq,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: GithubComChaitinPandaWikiProApiAuthV1AuthGroupSyncResp;
    }
  >({
    path: `/api/pro/v1/auth/group/sync`,
    method: "POST",
    body: param,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });
