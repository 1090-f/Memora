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
  DeleteApiProV1AuthGroupDeleteParams,
  DomainResponse,
  GetApiProV1AuthGroupDetailParams,
  GetApiProV1AuthGroupListParams,
  GetApiProV1AuthGroupTreeParams,
  GithubComChaitinPandaWikiProApiAuthV1AuthGroupCreateReq,
  GithubComChaitinPandaWikiProApiAuthV1AuthGroupCreateResp,
  GithubComChaitinPandaWikiProApiAuthV1AuthGroupDetailResp,
  GithubComChaitinPandaWikiProApiAuthV1AuthGroupListResp,
  GithubComChaitinPandaWikiProApiAuthV1AuthGroupMoveReq,
  GithubComChaitinPandaWikiProApiAuthV1AuthGroupTreeResp,
  GithubComChaitinPandaWikiProApiAuthV1AuthGroupUpdateReq,
} from "./types";

/**
 * @description 创建用户组
 *
 * @tags AuthGroup
 * @name PostApiProV1AuthGroupCreate
 * @summary 创建用户组
 * @request POST:/api/pro/v1/auth/group/create
 * @secure
 * @response `200` `(DomainResponse & {
    data?: GithubComChaitinPandaWikiProApiAuthV1AuthGroupCreateResp,

})` OK
 */

export const postApiProV1AuthGroupCreate = (
  param: GithubComChaitinPandaWikiProApiAuthV1AuthGroupCreateReq,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: GithubComChaitinPandaWikiProApiAuthV1AuthGroupCreateResp;
    }
  >({
    path: `/api/pro/v1/auth/group/create`,
    method: "POST",
    body: param,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 删除用户组
 *
 * @tags AuthGroup
 * @name DeleteApiProV1AuthGroupDelete
 * @summary 删除用户组
 * @request DELETE:/api/pro/v1/auth/group/delete
 * @secure
 * @response `200` `DomainResponse` OK
 */

export const deleteApiProV1AuthGroupDelete = (
  query: DeleteApiProV1AuthGroupDeleteParams,
  params: RequestParams = {},
) =>
  httpRequest<DomainResponse>({
    path: `/api/pro/v1/auth/group/delete`,
    method: "DELETE",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 获取用户组详情
 *
 * @tags AuthGroup
 * @name GetApiProV1AuthGroupDetail
 * @summary 获取用户组详情
 * @request GET:/api/pro/v1/auth/group/detail
 * @secure
 * @response `200` `(DomainResponse & {
    data?: GithubComChaitinPandaWikiProApiAuthV1AuthGroupDetailResp,

})` OK
 */

export const getApiProV1AuthGroupDetail = (
  query: GetApiProV1AuthGroupDetailParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: GithubComChaitinPandaWikiProApiAuthV1AuthGroupDetailResp;
    }
  >({
    path: `/api/pro/v1/auth/group/detail`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 获取用户组列表
 *
 * @tags AuthGroup
 * @name GetApiProV1AuthGroupList
 * @summary 获取用户组列表
 * @request GET:/api/pro/v1/auth/group/list
 * @secure
 * @response `200` `(DomainResponse & {
    data?: GithubComChaitinPandaWikiProApiAuthV1AuthGroupListResp,

})` OK
 */

export const getApiProV1AuthGroupList = (
  query: GetApiProV1AuthGroupListParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: GithubComChaitinPandaWikiProApiAuthV1AuthGroupListResp;
    }
  >({
    path: `/api/pro/v1/auth/group/list`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 移动用户组到新的父组下
 *
 * @tags AuthGroup
 * @name PatchApiProV1AuthGroupMove
 * @summary 移动用户组
 * @request PATCH:/api/pro/v1/auth/group/move
 * @secure
 * @response `200` `DomainResponse` OK
 */

export const patchApiProV1AuthGroupMove = (
  param: GithubComChaitinPandaWikiProApiAuthV1AuthGroupMoveReq,
  params: RequestParams = {},
) =>
  httpRequest<DomainResponse>({
    path: `/api/pro/v1/auth/group/move`,
    method: "PATCH",
    body: param,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 获取用户组树形结构
 *
 * @tags AuthGroup
 * @name GetApiProV1AuthGroupTree
 * @summary 获取用户组树形结构
 * @request GET:/api/pro/v1/auth/group/tree
 * @secure
 * @response `200` `(DomainResponse & {
    data?: GithubComChaitinPandaWikiProApiAuthV1AuthGroupTreeResp,

})` OK
 */

export const getApiProV1AuthGroupTree = (
  query: GetApiProV1AuthGroupTreeParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: GithubComChaitinPandaWikiProApiAuthV1AuthGroupTreeResp;
    }
  >({
    path: `/api/pro/v1/auth/group/tree`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 更新用户组名称和成员
 *
 * @tags AuthGroup
 * @name PatchApiProV1AuthGroupUpdate
 * @summary 更新用户组
 * @request PATCH:/api/pro/v1/auth/group/update
 * @secure
 * @response `200` `DomainResponse` OK
 */

export const patchApiProV1AuthGroupUpdate = (
  param: GithubComChaitinPandaWikiProApiAuthV1AuthGroupUpdateReq,
  params: RequestParams = {},
) =>
  httpRequest<DomainResponse>({
    path: `/api/pro/v1/auth/group/update`,
    method: "PATCH",
    body: param,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });
