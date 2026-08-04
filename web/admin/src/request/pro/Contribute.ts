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
  GetApiProV1ContributeDetailParams,
  GetApiProV1ContributeListParams,
  GithubComChaitinPandaWikiProApiContributeV1ContributeAuditReq,
  GithubComChaitinPandaWikiProApiContributeV1ContributeAuditResp,
  GithubComChaitinPandaWikiProApiContributeV1ContributeDetailResp,
  GithubComChaitinPandaWikiProApiContributeV1ContributeListResp,
} from "./types";

/**
 * @description 审核文档贡献，支持通过或拒绝
 *
 * @tags Contribute
 * @name PostApiProV1ContributeAudit
 * @summary 审核贡献
 * @request POST:/api/pro/v1/contribute/audit
 * @secure
 * @response `200` `(DomainResponse & {
    data?: GithubComChaitinPandaWikiProApiContributeV1ContributeAuditResp,

})` OK
 */

export const postApiProV1ContributeAudit = (
  param: GithubComChaitinPandaWikiProApiContributeV1ContributeAuditReq,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: GithubComChaitinPandaWikiProApiContributeV1ContributeAuditResp;
    }
  >({
    path: `/api/pro/v1/contribute/audit`,
    method: "POST",
    body: param,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 根据ID获取文档贡献详情
 *
 * @tags Contribute
 * @name GetApiProV1ContributeDetail
 * @summary 获取贡献详情
 * @request GET:/api/pro/v1/contribute/detail
 * @secure
 * @response `200` `(DomainResponse & {
    data?: GithubComChaitinPandaWikiProApiContributeV1ContributeDetailResp,

})` OK
 */

export const getApiProV1ContributeDetail = (
  query: GetApiProV1ContributeDetailParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: GithubComChaitinPandaWikiProApiContributeV1ContributeDetailResp;
    }
  >({
    path: `/api/pro/v1/contribute/detail`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 获取文档贡献列表，支持按知识库和状态筛选
 *
 * @tags Contribute
 * @name GetApiProV1ContributeList
 * @summary 获取贡献列表
 * @request GET:/api/pro/v1/contribute/list
 * @secure
 * @response `200` `(DomainResponse & {
    data?: GithubComChaitinPandaWikiProApiContributeV1ContributeListResp,

})` OK
 */

export const getApiProV1ContributeList = (
  query: GetApiProV1ContributeListParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: GithubComChaitinPandaWikiProApiContributeV1ContributeListResp;
    }
  >({
    path: `/api/pro/v1/contribute/list`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });
