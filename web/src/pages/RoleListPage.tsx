import * as React from "react";
import i18next from "i18next";
import {CrudListPage} from "@/components/crud/CrudListPage";
import {XlsxImport} from "@/components/crud/XlsxImport";
import {boolColumn, dateColumn, linkColumn, organizationColumn, refsColumn, tagsColumn, textColumn} from "@/components/crud/columns";
import type {ColumnDef} from "@/components/crud/types";
import {useAccount} from "@/hooks/use-account";
import * as Setting from "@/lib/setting";
import {useOrganizationFilter} from "@/hooks/use-organization";
import * as RoleBackend from "@/backend/RoleBackend";
import {newRole} from "@/pages/defaults";
import {useGroupList, useRoleList, useUserList} from "@/hooks/use-options";

export default function RoleListPage() {
  const {account} = useAccount();
  const organizationName = useOrganizationFilter();
  const groups = useGroupList(organizationName);
  const users = useUserList(organizationName);
  const roles = useRoleList(organizationName);
  const groupDisplayNames = React.useMemo(() => new Map(
    groups.map((group) => [`${group.owner}/${group.name}`, group.displayName || group.name]),
  ), [groups]);
  const userDisplayNames = React.useMemo(() => new Map(
    users.map((user) => [`${user.owner}/${user.name}`, user.displayName || user.name]),
  ), [users]);
  const roleDisplayNames = React.useMemo(() => new Map(
    roles.map((role) => [`${role.owner}/${role.name}`, role.displayName || role.name]),
  ), [roles]);

  const columns: ColumnDef<any>[] = [
    linkColumn({dataIndex: "name", to: (r) => `/roles/${r.owner}/${r.name}`}),
    organizationColumn(),
    dateColumn(),
    textColumn({dataIndex: "displayName", title: i18next.t("general:Display name"), searchable: true, width: 160}),
    refsColumn({
      dataIndex: "users",
      title: i18next.t("role:Sub users"),
      urlPrefix: "/users",
      width: 200,
      sortable: true,
      searchable: true,
      getLabel: (id) => userDisplayNames.get(id),
    }),
    refsColumn({
      dataIndex: "groups",
      title: i18next.t("role:Sub groups"),
      urlPrefix: "/groups",
      width: 180,
      sortable: true,
      searchable: true,
      getLabel: (id) => groupDisplayNames.get(id) || undefined,
    }),
    refsColumn({
      dataIndex: "roles",
      title: i18next.t("role:Sub roles"),
      urlPrefix: "/roles",
      width: 180,
      sortable: true,
      searchable: true,
      getLabel: (id) => roleDisplayNames.get(id),
    }),
    tagsColumn({dataIndex: "domains", title: i18next.t("role:Sub domains"), width: 160, sortable: true, searchable: true}),
    boolColumn({dataIndex: "isEnabled", title: i18next.t("general:Is enabled")}),
  ];

  return (
    <CrudListPage
      title={i18next.t("general:Roles")}
      columns={columns}
      toolbar={({refresh}) => (
        <XlsxImport
          columns={Setting.getRoleColumns()}
          templateName="import-role.xlsx"
          uploadApi="upload-roles"
          successMessage="Roles uploaded successfully, refreshing the page"
          onUploaded={refresh}
        />
      )}
      deps={[organizationName]}
      fetch={(q) =>
        RoleBackend.getRoles(organizationName, q.page, q.pageSize, q.searchedColumn, q.searchText, q.sortField, q.sortOrder)
      }
      newRecord={account ? () => newRole(account) : undefined}
      editUrl={(r) => `/roles/${r.owner}/${r.name}`}
      remove={(r) => RoleBackend.deleteRole(r)}
    />
  );
}
