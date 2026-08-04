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
  DeleteApiProV1TokenDeleteParams,
  DomainPWResponse,
  GetApiProV1TokenListParams,
  GithubComChaitinPandaWikiProApiTokenV1APITokenListItem,
  GithubComChaitinPandaWikiProApiTokenV1CreateAPITokenReq,
  GithubComChaitinPandaWikiProApiTokenV1UpdateAPITokenReq,
} from "./types";

/**
 * @description 创建 APIToken
 *
 * @tags ApiToken
 * @name PostApiProV1TokenCreate
 * @summary 创建 APIToken
 * @request POST:/api/pro/v1/token/create
 * @secure
 * @response `200` `DomainPWResponse` OK
 */

export const postApiProV1TokenCreate = (
  param: GithubComChaitinPandaWikiProApiTokenV1CreateAPITokenReq,
  params: RequestParams = {},
) =>
  httpRequest<DomainPWResponse>({
    path: `/api/pro/v1/token/create`,
    method: "POST",
    body: param,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 删除指定的API Token，需要full_control权限
 *
 * @tags ApiToken
 * @name DeleteApiProV1TokenDelete
 * @summary 删除API Token
 * @request DELETE:/api/pro/v1/token/delete
 * @secure
 * @response `200` `DomainPWResponse` OK
 */

export const deleteApiProV1TokenDelete = (
  query: DeleteApiProV1TokenDeleteParams,
  params: RequestParams = {},
) =>
  httpRequest<DomainPWResponse>({
    path: `/api/pro/v1/token/delete`,
    method: "DELETE",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 获取当前用户的所有API Token列表，需要full_control权限
 *
 * @tags ApiToken
 * @name GetApiProV1TokenList
 * @summary 获取API Token列表
 * @request GET:/api/pro/v1/token/list
 * @secure
 * @response `200` `(DomainPWResponse & {
    data?: (GithubComChaitinPandaWikiProApiTokenV1APITokenListItem)[],

})` OK
 */

export const getApiProV1TokenList = (
  query: GetApiProV1TokenListParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainPWResponse & {
      data?: GithubComChaitinPandaWikiProApiTokenV1APITokenListItem[];
    }
  >({
    path: `/api/pro/v1/token/list`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 更新API Token的名称和权限，需要full_control权限
 *
 * @tags ApiToken
 * @name PatchApiProV1TokenUpdate
 * @summary 更新API Token
 * @request PATCH:/api/pro/v1/token/update
 * @secure
 * @response `200` `DomainPWResponse` OK
 */

export const patchApiProV1TokenUpdate = (
  request: GithubComChaitinPandaWikiProApiTokenV1UpdateAPITokenReq,
  params: RequestParams = {},
) =>
  httpRequest<DomainPWResponse>({
    path: `/api/pro/v1/token/update`,
    method: "PATCH",
    body: request,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });
