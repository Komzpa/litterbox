# Litterbox Qt client shell: network API object, card store model,
# page-loading hook and fallback inbox view. Page content under qt/pages
# is owned by another slice; this shell only loads it when present.
cmake_minimum_required(VERSION 3.20)
project(litterbox-qt VERSION 0.1 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_package(Qt6 6.5 REQUIRED COMPONENTS Core Gui Qml Quick Network Sql Test QuickControls2)
find_package(KF6Kirigami CONFIG REQUIRED)

qt_standard_project_setup(REQUIRES 6.5)

enable_testing()

qt_add_executable(litterbox-qt
    src/main.cpp
    src/api.h
    src/api.cpp
    src/androidupdater.h
    src/androidupdater.cpp
    src/CardStore.h
    src/CardStore.cpp
    src/timerules.h
    src/timerules.cpp
)
target_compile_definitions(litterbox-qt PRIVATE LB_SOURCE_PAGES_DIR="${CMAKE_CURRENT_SOURCE_DIR}/pages")
set_property(TARGET litterbox-qt PROPERTY QT_ANDROID_PACKAGE_SOURCE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/android")
# Compiled-in default server URL. Android has no LB_SERVER environment, so
# the phone uses this until enrollment saves a server_url to QSettings.
# R17: the repo default stays empty (a LAN address is a deployment detail);
# release builds pass the real URL with -DLB_DEFAULT_SERVER_URL=<url>.
set(LB_DEFAULT_SERVER_URL "" CACHE STRING "Compiled-in default Litterbox server URL")
target_compile_definitions(litterbox-qt PRIVATE LB_DEFAULT_SERVER_URL="${LB_DEFAULT_SERVER_URL}")

# Page content under qt/pages is owned by the QML slice; the shell only loads
# it when present. Embed the same files as raw resources as well: on Android
# the compile-time LB_SOURCE_PAGES_DIR path does not exist, so the app falls
# back to this qrc prefix there. Raw resources, not QML_FILES, so page code
# stays owned by its slice (no type registration or lint of it here).
qt_add_resources(litterbox-qt "lb_pages"
    PREFIX "/qt/qml/litterbox/qml"
    FILES
        pages/EnrollmentPage.qml
        pages/GmailAccountsPage.qml
        pages/JournalPage.qml
        pages/McpTokensPage.qml
        pages/MailDetailPage.qml
        pages/ReminderDialog.qml
)


set_source_files_properties(src/qml/CardActions.qml PROPERTIES QT_RESOURCE_ALIAS qml/CardActions.qml)

qt_add_qml_module(litterbox-qt
    URI litterbox
    VERSION 1.0
    QML_FILES
        qml/InboxView.qml
        src/qml/CardActions.qml
)

set(LB_KIRIGAMI_QML_IMPORT_PATH "" CACHE PATH "Kirigami QML import root for Android deployment")
if(NOT LB_KIRIGAMI_QML_IMPORT_PATH AND KF6Kirigami_DIR)
    # Android builds install Kirigami into a standalone prefix whose QML tree
    # sits outside Qt's own import path. Without it the QML import scanner
    # leaves org.kde.kirigami unresolved (no PATH/PLUGIN) and the APK omits
    # the Kirigami QML plugin. Derive the root from the Kirigami package
    # location unless the caller passes -DLB_KIRIGAMI_QML_IMPORT_PATH.
    get_filename_component(_lb_kirigami_qml_root "${KF6Kirigami_DIR}/../../qml" ABSOLUTE)
    if(EXISTS "${_lb_kirigami_qml_root}/org/kde/kirigami/qmldir")
        set(LB_KIRIGAMI_QML_IMPORT_PATH "${_lb_kirigami_qml_root}")
    endif()
endif()
if(LB_KIRIGAMI_QML_IMPORT_PATH)
    set_property(TARGET litterbox-qt PROPERTY QT_QML_IMPORT_PATH "${LB_KIRIGAMI_QML_IMPORT_PATH}")
endif()
target_link_libraries(litterbox-qt PRIVATE
    Qt6::Core
    Qt6::Gui
    Qt6::Qml
    Qt6::Quick
    Qt6::Network
    Qt6::Sql
    Qt6::QuickControls2
    KF6::Kirigami
)

# Reuse the canonical CardStore's isolated QtTest target.
add_subdirectory(tests)

qt_add_executable(tst_timerules tests/tst_timerules.cpp src/timerules.h src/timerules.cpp)
target_include_directories(tst_timerules PRIVATE src)
target_link_libraries(tst_timerules PRIVATE Qt6::Core Qt6::Test)
add_test(NAME timerules COMMAND tst_timerules)

qt_add_executable(tst_api tests/tst_api.cpp src/api.h src/api.cpp)
target_include_directories(tst_api PRIVATE src)
target_link_libraries(tst_api PRIVATE Qt6::Core Qt6::Qml Qt6::Network Qt6::Test)
add_test(NAME api COMMAND tst_api)
qt_add_executable(tst_androidupdater tests/tst_androidupdater.cpp src/androidupdater.h src/androidupdater.cpp)
target_include_directories(tst_androidupdater PRIVATE src)
target_link_libraries(tst_androidupdater PRIVATE Qt6::Core Qt6::Network Qt6::Test)
qt_add_executable(tst_serverurl tests/tst_serverurl.cpp src/api.h src/api.cpp)
target_include_directories(tst_serverurl PRIVATE src)
target_link_libraries(tst_serverurl PRIVATE Qt6::Core Qt6::Qml Qt6::Network Qt6::Test)
add_test(NAME serverurl COMMAND tst_serverurl)
add_test(NAME androidupdater COMMAND tst_androidupdater)


# Page slice QMLTestUnit specs (owned by the screens slice), run headless.
file(GLOB _lb_page_tests "${CMAKE_CURRENT_SOURCE_DIR}/tests/tst_*.qml")
list(REMOVE_ITEM _lb_page_tests "${CMAKE_CURRENT_SOURCE_DIR}/tests/tst_touchdefects.qml")
find_program(QMLTESTRUNNER6 NAMES qmltestrunner PATHS "/usr/lib/qt6/bin" NO_DEFAULT_PATH)
if (QMLTESTRUNNER6)
    foreach (_test ${_lb_page_tests})
        get_filename_component(_test_name ${_test} NAME_WE)
        add_test(NAME ${_test_name}
                 COMMAND ${QMLTESTRUNNER6} -input ${_test} -platform offscreen)
        set_tests_properties(${_test_name} PROPERTIES WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR})
    endforeach()
    add_test(NAME tst_touchdefects
             COMMAND ${QMLTESTRUNNER6} -input ${CMAKE_CURRENT_SOURCE_DIR}/tests/tst_touchdefects.qml
                     -import ${CMAKE_CURRENT_SOURCE_DIR}/tests/qml-imports -platform offscreen)
    set_tests_properties(tst_touchdefects PROPERTIES WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR})
endif()

# Install desktop executables to bin and Android module libraries to lib.
install(TARGETS litterbox-qt RUNTIME DESTINATION bin LIBRARY DESTINATION lib)
install(FILES debian/litterbox-qt.desktop DESTINATION share/applications)
install(FILES debian/litterbox-qt.svg DESTINATION share/icons/hicolor/scalable/apps)
